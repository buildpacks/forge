package forge

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"text/template"
	"time"

	"github.com/buildpack/forge/engine"
	"github.com/buildpack/forge/internal"
)

const forwardScriptTmpl = `
	{{if .Forwards -}}
	echo 'Forwarding:{{range .Forwards}} {{.Name}}{{end}}'
	sshpass -f /tmp/ssh-code ssh -4 -N \
	    -o PermitLocalCommand=yes -o LocalCommand="touch /tmp/healthy" \
		-o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no \
		-o LogLevel=ERROR -o ExitOnForwardFailure=yes \
		-o ServerAliveInterval=10 -o ServerAliveCountMax=60 \
		-p '{{.Port}}' '{{.User}}@{{.Host}}' \
		{{- range $i, $_ := .Forwards}}
		{{- if $i}} \{{end}}
		-L '{{.From}}:{{.To}}'
		{{- end}}
	rm -f /tmp/healthy
	{{- end}}
`

type Forwarder struct {
	Logs   io.Writer
	engine Engine
}

type ForwardConfig struct {
	AppName          string
	Stack            string
	Color            Colorizer
	Details          *ForwardDetails
	HostIP, HostPort string
	Wait             <-chan time.Time
}

func NewForwarder(engine Engine) *Forwarder {
	return &Forwarder{
		Logs:   os.Stdout,
		engine: engine,
	}
}

func (f *Forwarder) Forward(config *ForwardConfig) (health <-chan string, done func(), id string, err error) {
	output := internal.NewLockWriter(f.Logs)

	netContr, err := f.engine.NewContainer(f.buildNetConfig(config.AppName, config.Stack, config.HostIP, config.HostPort))
	if err != nil {
		return nil, nil, "", err
	}
	// TODO: wait for network container to fully start in Background
	if err := netContr.Background(); err != nil {
		return nil, nil, "", err
	}

	containerConfig, err := f.buildConfig(config.Details, config.Stack, netContr.ID())
	if err != nil {
		return nil, nil, "", err
	}
	contr, err := f.engine.NewContainer(containerConfig)
	if err != nil {
		return nil, nil, "", err
	}

	prefix := config.Color("[%s tunnel] ", config.AppName)
	wait := config.Wait
	exit := make(chan struct{})
	go func() {
		for {
			select {
			case <-exit:
				return
			case <-wait:
				code, err := config.Details.Code()
				if err != nil {
					fmt.Fprintf(output, "%sError: %s\n", prefix, err)
					continue
				}
				codeStream := engine.NewStream(ioutil.NopCloser(bytes.NewBufferString(code)), int64(len(code)))
				if err := contr.StreamFileTo(codeStream, "/tmp/ssh-code"); err != nil {
					fmt.Fprintf(output, "%sError: %s\n", prefix, err)
					continue
				}
				status, err := contr.Start(prefix, output, nil)
				if err != nil {
					fmt.Fprintf(output, "%sError: %s\n", prefix, err)
					continue
				}
				fmt.Fprintf(output, "%sExited with status: %d\n", prefix, status)
			}
		}
	}()
	done = func() {
		defer netContr.Close()
		defer contr.Close()
		wait = nil
		close(exit)
		output.Disable()
	}
	return contr.HealthCheck(), done, netContr.ID(), nil
}

func (f *Forwarder) buildConfig(forward *ForwardDetails, stack, netID string) (*engine.ContainerConfig, error) {
	scriptBuf := &bytes.Buffer{}
	tmpl := template.Must(template.New("").Parse(forwardScriptTmpl))
	if err := tmpl.Execute(scriptBuf, forward); err != nil {
		return nil, err
	}

	return &engine.ContainerConfig{
		Name:         "service",
		Image:        stack,
		Entrypoint:   []string{"/bin/bash", "-c", scriptBuf.String()},
		NetContainer: netID,
		Test:         []string{"CMD", "test", "-f", "/tmp/healthy"},
		Interval:     time.Second,
		Retries:      30,
		Exit:         make(<-chan struct{}),
	}, nil
}

func (f *Forwarder) buildNetConfig(name, stack, hostIP, hostPort string) *engine.ContainerConfig {
	return &engine.ContainerConfig{
		Name:       "network",
		Hostname:   name,
		Image:      stack,
		Entrypoint: []string{"tail", "-f", "/dev/null"},
		HostIP:     hostIP,
		HostPort:   hostPort,
		Exit:       make(<-chan struct{}),
	}
}
