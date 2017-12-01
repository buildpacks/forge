package forge

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"text/template"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	docker "github.com/docker/docker/client"

	"github.com/docker/go-connections/nat"
	"github.com/sclevine/forge/engine"
	"github.com/sclevine/forge/internal"
)

const forwardScript = `
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
	engine forgeEngine
}

type ForwardConfig struct {
	AppName          string
	Stack            string
	SSHPass          engine.Stream
	Color            Colorizer
	Details          *ForwardDetails
	HostIP, HostPort string
	Wait             <-chan time.Time
}

func NewForwarder(client *docker.Client) *Forwarder {
	return &Forwarder{
		Logs: os.Stdout,
		engine: &dockerEngine{
			Docker: client,
		},
	}
}

func (f *Forwarder) Forward(config *ForwardConfig) (health <-chan string, done func(), id string, err error) {
	output := internal.NewLockWriter(f.Logs)

	netHostConfig := &container.HostConfig{PortBindings: nat.PortMap{
		"8080/tcp": {{HostIP: config.HostIP, HostPort: config.HostPort}},
	}}
	netContr, err := f.engine.NewContainer("network", f.buildNetContainerConfig(config.AppName, config.Stack), netHostConfig)
	if err != nil {
		return nil, nil, "", err
	}
	// TODO: wait for network container to fully start in Background
	if err := netContr.Background(); err != nil {
		return nil, nil, "", err
	}

	networkMode := "container:" + netContr.ID()
	containerConfig, err := f.buildContainerConfig(config.Details, config.Stack)
	if err != nil {
		return nil, nil, "", err
	}
	hostConfig := &container.HostConfig{NetworkMode: container.NetworkMode(networkMode)}
	contr, err := f.engine.NewContainer("service", containerConfig, hostConfig)
	if err != nil {
		return nil, nil, "", err
	}

	if err := contr.StreamFileTo(config.SSHPass, "/usr/bin/sshpass"); err != nil {
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

func (f *Forwarder) buildContainerConfig(forwardConfig *ForwardDetails, stack string) (*container.Config, error) {
	scriptBuf := &bytes.Buffer{}
	tmpl := template.Must(template.New("").Parse(forwardScript))
	if err := tmpl.Execute(scriptBuf, forwardConfig); err != nil {
		return nil, err
	}

	return &container.Config{
		User: "vcap",
		Healthcheck: &container.HealthConfig{
			Test:     []string{"CMD", "test", "-f", "/tmp/healthy"},
			Interval: time.Second,
			Retries:  30,
		},
		Image: stack,
		Entrypoint: strslice.StrSlice{
			"/bin/bash", "-c", scriptBuf.String(),
		},
	}, nil
}

func (f *Forwarder) buildNetContainerConfig(name string, stack string) *container.Config {
	return &container.Config{
		Hostname:     name,
		User:         "vcap",
		ExposedPorts: nat.PortSet{"8080/tcp": {}},
		Image:        stack,
		Entrypoint: strslice.StrSlice{
			"tail", "-f", "/dev/null",
		},
	}
}
