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
	"github.com/sclevine/forge/service"
)

const runnerScript = `
	set -e
	{{if .RSync -}}
	rsync -a /tmp/local/ /home/vcap/app/
	{{end -}}
	if [[ ! -z $(ls -A /home/vcap/app) ]]; then
		exclude='--exclude=./app'
	fi
	tar $exclude -C /home/vcap -xzf /tmp/droplet
	chown -R vcap:vcap /home/vcap
	{{if .RSync -}}
	if [[ -z $(ls -A /tmp/local) ]]; then
		rsync -a /home/vcap/app/ /tmp/local/
	fi
	{{end -}}
	command=$1
	if [[ -z $command ]]; then
		command=$(jq -r .start_command /home/vcap/staging_info.yml)
	fi
	exec /tmp/lifecycle/launcher /home/vcap/app "$command" ''
`

var bytesPattern = regexp.MustCompile(`(?i)^(-?\d+)([KMGT])B?$`)

type Runner struct {
	Logs   io.Writer
	Loader Loader
	engine forgeEngine
	image  forgeImage
}

type RunConfig struct {
	Droplet       engine.Stream
	Launcher      engine.Stream
	Stack         string
	AppDir        string
	RSync         bool
	Restart       <-chan time.Time
	Color         Colorizer
	AppConfig     *AppConfig
	NetworkConfig *NetworkConfig
}

func NewRunner(client *docker.Client, exit <-chan struct{}) *Runner {
	return &Runner{
		Logs:   os.Stdout,
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

	r.setDefaults(config.AppConfig)
	containerConfig, err := r.buildContainerConfig(config.AppConfig, config.Stack, config.RSync, config.NetworkConfig.ContainerID != "")
	if err != nil {
		return 0, err
	}
	remoteDir := "/home/vcap/app"
	if config.RSync {
		remoteDir = "/tmp/local"
	}
	memory, err := toMegabytes(config.AppConfig.Memory)
	if err != nil {
		return 0, err
	}
	hostConfig := r.buildHostConfig(config.NetworkConfig, memory, config.AppDir, remoteDir)
	contr, err := r.engine.NewContainer(config.AppConfig.Name, containerConfig, hostConfig)
	if err != nil {
		return 0, err
	}
	defer contr.Close()

	if err := contr.CopyTo(config.Launcher, "/tmp/lifecycle/launcher"); err != nil {
		return 0, err
	}
	if err := contr.CopyTo(config.Droplet, "/tmp/droplet"); err != nil {
		return 0, err
	}
	return contr.Start(config.Color("[%s] ", config.AppConfig.Name), r.Logs, config.Restart)
}

type ExportConfig struct {
	Droplet   engine.Stream
	Launcher  engine.Stream
	Stack     string
	Ref       string
	AppConfig *AppConfig
}

func (r *Runner) Export(config *ExportConfig) (imageID string, err error) {
	if err := r.pull(config.Stack); err != nil {
		return "", err
	}

	r.setDefaults(config.AppConfig)
	containerConfig, err := r.buildContainerConfig(config.AppConfig, config.Stack, false, false)
	if err != nil {
		return "", err
	}
	contr, err := r.engine.NewContainer(config.AppConfig.Name, containerConfig, nil)
	if err != nil {
		return "", err
	}
	defer contr.Close()

	if err := contr.CopyTo(config.Launcher, "/tmp/lifecycle/launcher"); err != nil {
		return "", err
	}
	if err := contr.CopyTo(config.Droplet, "/tmp/droplet"); err != nil {
		return "", err
	}

	return contr.Commit(config.Ref)
}

func (r *Runner) pull(stack string) error {
	return r.Loader.Loading("Image", r.image.Pull(stack))
}

func (r *Runner) setDefaults(config *AppConfig) {
	if config.Memory == "" {
		config.Memory = "1024m"
	}
	if config.DiskQuota == "" {
		config.DiskQuota = "1024m"
	}
}

func (r *Runner) buildContainerConfig(config *AppConfig, stack string, rsync, networked bool) (*container.Config, error) {
	name := config.Name
	memory, err := toMegabytes(config.Memory)
	if err != nil {
		return nil, err
	}
	disk, err := toMegabytes(config.DiskQuota)
	if err != nil {
		return nil, err
	}
	vcapApp, err := json.Marshal(&vcapApplication{
		ApplicationID:      "01d31c12-d066-495e-aca2-8d3403165360",
		ApplicationName:    name,
		ApplicationURIs:    []string{"localhost"},
		ApplicationVersion: "2b860df9-a0a1-474c-b02f-5985f53ea0bb",
		Host:               "0.0.0.0",
		InstanceID:         "999db41a-508b-46eb-74d8-6f9c06c006da",
		InstanceIndex:      uintPtr(0),
		Limits:             map[string]int64{"fds": 16384, "mem": memory, "disk": disk},
		Name:               name,
		Port:               uintPtr(8080),
		SpaceID:            "18300c1c-1aa4-4ae7-81e6-ae59c6cdbaf1",
		SpaceName:          config.Name + "-space",
		URIs:               []string{"localhost"},
		Version:            "18300c1c-1aa4-4ae7-81e6-ae59c6cdbaf1",
	})
	if err != nil {
		return nil, err
	}

	services := config.Services
	if services == nil {
		services = service.Services{}
	}
	vcapServices, err := json.Marshal(services)
	if err != nil {
		return nil, err
	}

	env := map[string]string{
		"CF_INSTANCE_ADDR":  "0.0.0.0:8080",
		"CF_INSTANCE_GUID":  "999db41a-508b-46eb-74d8-6f9c06c006da",
		"CF_INSTANCE_INDEX": "0",
		"CF_INSTANCE_IP":    "0.0.0.0",
		"CF_INSTANCE_PORT":  "8080",
		"CF_INSTANCE_PORTS": `[{"external":8080,"internal":8080}]`,
		"INSTANCE_GUID":     "999db41a-508b-46eb-74d8-6f9c06c006da",
		"INSTANCE_INDEX":    "0",
		"LANG":              "en_US.UTF-8",
		"MEMORY_LIMIT":      fmt.Sprintf("%dm", memory),
		"PATH":              "/usr/local/bin:/usr/bin:/bin",
		"PORT":              "8080",
		"TMPDIR":            "/home/vcap/tmp",
		"USER":              "vcap",
		"VCAP_APPLICATION":  string(vcapApp),
		"VCAP_SERVICES":     string(vcapServices),
	}

	options := struct{ RSync bool }{rsync}
	scriptBuf := &bytes.Buffer{}
	tmpl := template.Must(template.New("").Parse(runnerScript))
	if err := tmpl.Execute(scriptBuf, options); err != nil {
		return nil, err
	}

	hostname := config.Name
	ports := nat.PortSet{"8080/tcp": {}}

	if networked {
		hostname = ""
		ports = nil
	}

	return &container.Config{
		Hostname:     hostname,
		User:         "vcap",
		ExposedPorts: ports,
		Env:          mapToEnv(mergeMaps(env, config.RunningEnv, config.Env)),
		Image:        stack,
		WorkingDir:   "/home/vcap/app",
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

func uintPtr(i uint) *uint {
	return &i
}
