package forge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	docker "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/sclevine/forge/engine"
	"github.com/sclevine/forge/term"
)

const runScript = `
	set -e
	{{if .RSync -}}
	rsync -a /tmp/local/ /home/vcap/app/
	{{end -}}
	exec /launcher "$1"
`

var bytesPattern = regexp.MustCompile(`(?i)^(-?\d+)([KMGT])B?$`)

type Runner struct {
	Logs   io.Writer
	TTY    engine.TTY
	Loader Loader
	engine forgeEngine
	image  forgeImage
}

type RunConfig struct {
	Droplet       engine.Stream
	Stack         string
	AppDir        string
	RSync         bool
	Shell         bool
	Restart       <-chan time.Time
	Color         Colorizer
	AppConfig     *AppConfig
	NetworkConfig *NetworkConfig
}

func NewRunner(client *docker.Client, exit <-chan struct{}) *Runner {
	return &Runner{
		Logs: os.Stdout,
		TTY: &term.TTY{
			In:  os.Stdin,
			Out: os.Stdout,
		},
		Loader: noopLoader{},
		engine: &dockerEngine{
			Docker: client,
			Exit:   exit,
		},
		image: &engine.Image{
			Docker: client,
			Exit:   exit,
		},
	}
}

func (r *Runner) Run(config *RunConfig) (status int64, err error) {
	if err := r.pull(config.Stack); err != nil {
		return 0, err
	}

	containerConfig, err := r.buildContainerConfig(config.AppConfig, config.Stack, config.RSync, config.NetworkConfig.ContainerID != "")
	if err != nil {
		return 0, err
	}
	remoteDir := "/home/vcap/app"
	if config.RSync {
		remoteDir = "/tmp/local"
	}
	var memory int64
	if config.AppConfig.Memory != "" {
		memory, err = toMegabytes(config.AppConfig.Memory)
		if err != nil {
			return 0, err
		}
	}
	hostConfig := r.buildHostConfig(config.NetworkConfig, memory, config.AppDir, remoteDir)
	contr, err := r.engine.NewContainer(config.AppConfig.Name, containerConfig, hostConfig)
	if err != nil {
		return 0, err
	}
	defer contr.Close()

	if err := contr.StreamTarTo(config.Droplet, "/home/vcap"); err != nil {
		return 0, err
	}
	color := config.Color("[%s] ", config.AppConfig.Name)
	if !config.Shell {
		return contr.Start(color, r.Logs, config.Restart)
	}
	if err := contr.Background(); err != nil {
		return 0, err
	}
	return 0, contr.Shell(r.TTY, "/lifecycle/shell")
}

type ExportConfig struct {
	Droplet   engine.Stream
	Stack     string
	Ref       string
	AppConfig *AppConfig
}

// TODO: use build instead of commit
func (r *Runner) Export(config *ExportConfig) (imageID string, err error) {
	if err := r.pull(config.Stack); err != nil {
		return "", err
	}

	appConfig := config.AppConfig
	appConfig.DiskQuota = ""
	appConfig.Memory = ""
	containerConfig, err := r.buildContainerConfig(appConfig, config.Stack, false, false)
	if err != nil {
		return "", err
	}
	contr, err := r.engine.NewContainer(config.AppConfig.Name, containerConfig, nil)
	if err != nil {
		return "", err
	}
	defer contr.Close()

	if err := contr.StreamTarTo(config.Droplet, "/home/vcap"); err != nil {
		return "", err
	}
	return contr.Commit(config.Ref)
}

func (r *Runner) pull(stack string) error {
	return r.Loader.Loading("Image", r.image.Pull(stack))
}

func (r *Runner) buildContainerConfig(config *AppConfig, stack string, rsync, networked bool) (*container.Config, error) {
	env := map[string]string{}

	if config.Name != "" {
		env["PACK_APP_NAME"] = config.Name
	}

	if config.DiskQuota != "" {
		disk, err := toMegabytes(config.DiskQuota)
		if err != nil {
			return nil, err
		}
		env["PACK_APP_DISK"] = fmt.Sprintf("%d", disk)
	}
	if config.Memory != "" {
		mem, err := toMegabytes(config.Memory)
		if err != nil {
			return nil, err
		}
		env["PACK_APP_MEM"] = fmt.Sprintf("%d", mem)
	}

	if config.Services != nil {
		vcapServices, err := json.Marshal(config.Services)
		if err != nil {
			return nil, err
		}
		env["VCAP_SERVICES"] = string(vcapServices)
	}

	options := struct{ RSync bool }{rsync}
	scriptBuf := &bytes.Buffer{}
	tmpl := template.Must(template.New("").Parse(runScript))
	if err := tmpl.Execute(scriptBuf, options); err != nil {
		return nil, err
	}

	hostname := config.Name
	if networked {
		hostname = ""
	}
	return &container.Config{
		Hostname:   hostname,
		Env:        mapToEnv(mergeMaps(env, config.RunningEnv, config.Env)),
		Image:      stack,
		WorkingDir: "/home/vcap/app",
		Entrypoint: strslice.StrSlice{
			"/bin/bash", "-c", scriptBuf.String(), config.Command,
		},
	}, nil
}

func (*Runner) buildHostConfig(netConfig *NetworkConfig, memory int64, appDir, remoteDir string) *container.HostConfig {
	config := &container.HostConfig{
		Resources: container.Resources{
			Memory: memory * 1024 * 1024,
		},
	}
	if netConfig.ContainerID == "" {
		config.PortBindings = nat.PortMap{
			"8080/tcp": {{HostIP: netConfig.HostIP, HostPort: netConfig.HostPort}},
		}
	} else {
		config.NetworkMode = container.NetworkMode("container:" + netConfig.ContainerID)
	}
	if appDir != "" && remoteDir != "" {
		config.Binds = []string{appDir + ":" + remoteDir}
	}
	return config
}

func toMegabytes(s string) (int64, error) {
	parts := bytesPattern.FindStringSubmatch(strings.TrimSpace(s))
	if len(parts) < 3 {
		return 0, fmt.Errorf("invalid byte unit format: %s", s)
	}

	value, err := strconv.ParseInt(parts[1], 10, 0)
	if err != nil {
		return 0, fmt.Errorf("invalid byte number format: %s", s)
	}

	const (
		kilobyte = 1024
		megabyte = 1024 * kilobyte
		gigabyte = 1024 * megabyte
		terabyte = 1024 * gigabyte
	)

	var bytes int64
	switch strings.ToUpper(parts[2]) {
	case "T":
		bytes = value * terabyte
	case "G":
		bytes = value * gigabyte
	case "M":
		bytes = value * megabyte
	case "K":
		bytes = value * kilobyte
	}
	return bytes / megabyte, nil
}

func mergeMaps(maps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

func mapToEnv(env map[string]string) []string {
	var out []string
	for k, v := range env {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}
