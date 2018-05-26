package forge

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"path/filepath"

	"github.com/sclevine/forge/engine"
	"github.com/sclevine/forge/engine/docker/term"
)

const runScript = `
	set -eo pipefail
	if [[ -d /tmp/local ]]; then
		rsync -a --delete /tmp/local/ /home/vcap/app
    fi
	exec /packs/launcher "$1"
`

var bytesPattern = regexp.MustCompile(`(?i)^(-?\d+)([KMGT])B?$`)

type Runner struct {
	Logs   io.Writer
	TTY    engine.TTY
	Loader Loader
	engine Engine
}

type RunConfig struct {
	Droplet       engine.Stream
	Stack         string
	AppDir        string
	Shell         bool
	Restart       <-chan time.Time
	Color         Colorizer
	AppConfig     *AppConfig
	NetworkConfig *NetworkConfig
	SkipStackPull bool
	RootDir       string
}

func NewRunner(engine Engine) *Runner {
	return &Runner{
		Logs: os.Stdout,
		TTY: &term.TTY{
			In:  os.Stdin,
			Out: os.Stdout,
		},
		Loader: noopLoader{},
		engine: engine,
	}
}

func (r *Runner) Run(config *RunConfig) (status int64, err error) {
	if config.SkipStackPull == false {
		if err := r.pull(config.Stack); err != nil {
			return 0, err
		}
	}

	rootDir := config.RootDir
	if rootDir == "" {
		rootDir = "/home/vcap"
	}

	homeDir := filepath.Join(rootDir, "app")

	var binds []string
	if config.AppDir != "" {
		binds = []string{config.AppDir + ":/tmp/local"}
	}
	containerConfig, err := r.buildConfig(config.AppConfig, config.NetworkConfig, binds, config.Stack, homeDir)
	if err != nil {
		return 0, err
	}
	contr, err := r.engine.NewContainer(containerConfig)
	if err != nil {
		return 0, err
	}
	defer contr.Close()

	if err := contr.StreamTarTo(config.Droplet, rootDir); err != nil {
		return 0, err
	}
	color := config.Color("[%s] ", config.AppConfig.Name)
	if !config.Shell {
		return contr.Start(color, r.Logs, config.Restart)
	}
	if err := contr.Background(); err != nil {
		return 0, err
	}
	return 0, contr.Shell(r.TTY, "/packs/shell")
}

func (r *Runner) pull(stack string) error {
	return r.Loader.Loading("Image", r.engine.NewImage().Pull(stack))
}

func (r *Runner) buildConfig(app *AppConfig, net *NetworkConfig, binds []string, stack string, workDir string) (*engine.ContainerConfig, error) {
	var disk, mem int64
	var err error
	env := map[string]string{}

	if app.Name != "" {
		env["PACK_APP_NAME"] = app.Name
	}

	if app.DiskQuota != "" {
		disk, err = toMegabytes(app.DiskQuota)
		if err != nil {
			return nil, err
		}
		env["PACK_APP_DISK"] = fmt.Sprintf("%d", disk)
	}
	if app.Memory != "" {
		mem, err = toMegabytes(app.Memory)
		if err != nil {
			return nil, err
		}
		env["PACK_APP_MEM"] = fmt.Sprintf("%d", mem)
	}

	if app.Services != nil {
		vcapServices, err := json.Marshal(app.Services)
		if err != nil {
			return nil, err
		}
		env["VCAP_SERVICES"] = string(vcapServices)
	}

	return &engine.ContainerConfig{
		Name:       app.Name,
		Hostname:   app.Name,
		Env:        mapToEnv(mergeMaps(env, app.RunningEnv, app.Env)),
		Image:      stack,
		WorkingDir: workDir,
		Entrypoint: []string{"/bin/bash", "-c", runScript, app.Command},

		Binds:        binds,
		NetContainer: net.ContainerID,
		HostIP:       net.HostIP,
		HostPort:     net.HostPort,
		Memory:       mem * 1024 * 1024,
		DiskQuota:    disk * 1024 * 1024,
	}, nil
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
