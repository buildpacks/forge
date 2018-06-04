package forge

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/sclevine/forge/engine"
)

type Stager struct {
	Logs   io.Writer
	engine Engine
}

type StageConfig struct {
	AppTar        io.Reader
	Cache         ReadResetWriter
	CacheEmpty    bool
	BuildpackZips map[string]engine.Stream
	Stack         string
	OutputPath    string
	ForceDetect   bool
	Color         Colorizer
	AppConfig     *AppConfig
}

type ReadResetWriter interface {
	io.ReadWriter
	Reset() error
}

func NewStager(engine Engine) *Stager {
	return &Stager{
		Logs:   os.Stdout,
		engine: engine,
	}
}

func (s *Stager) Stage(config *StageConfig) (droplet engine.Stream, err error) {
	containerConfig, err := s.buildConfig(config.AppConfig, config.Stack, config.ForceDetect)
	if err != nil {
		return engine.Stream{}, err
	}
	contr, err := s.engine.NewContainer(containerConfig)
	if err != nil {
		return engine.Stream{}, err
	}
	defer contr.CloseAfterStream(&droplet)

	for checksum, zip := range config.BuildpackZips {
		if err := contr.StreamFileTo(zip, fmt.Sprintf("/buildpacks/%s.zip", checksum)); err != nil {
			return engine.Stream{}, err
		}
	}

	if err := contr.UploadTarTo(config.AppTar, "/tmp/app"); err != nil {
		return engine.Stream{}, err
	}

	if !config.CacheEmpty {
		if err := contr.Mkdir("/tmp/cache"); err != nil {
			return engine.Stream{}, err
		}
		if err := contr.UploadTarTo(config.Cache, "/tmp/cache"); err != nil {
			return engine.Stream{}, err
		}
	}

	status, err := contr.Start(config.Color("[%s] ", config.AppConfig.Name), s.Logs, nil)
	if err != nil {
		return engine.Stream{}, err
	}
	if status != 0 {
		return engine.Stream{}, fmt.Errorf("container exited with status %d", status)
	}

	if err := config.Cache.Reset(); err != nil {
		return engine.Stream{}, err
	}
	if err := streamOut(contr, config.Cache, "/cache/cache.tgz"); err != nil {
		return engine.Stream{}, err
	}

	return contr.StreamFileFrom(config.OutputPath)
}

func (s *Stager) buildConfig(app *AppConfig, stack string, forceDetect bool) (*engine.ContainerConfig, error) {
	var (
		buildpacks []string
		detect     bool
	)
	if app.Buildpack == "" && len(app.Buildpacks) == 0 {
		detect = true
	} else if len(app.Buildpacks) > 0 {
		buildpacks = app.Buildpacks
	} else {
		buildpacks = []string{app.Buildpack}
	}
	detect = detect || forceDetect
	if detect {
		fmt.Fprintln(s.Logs, "Buildpack: will detect")
	} else {
		var plurality string
		if len(buildpacks) > 1 {
			plurality = "s"
		}
		fmt.Fprintf(s.Logs, "Buildpack%s: %s\n", plurality, strings.Join(buildpacks, ", "))
	}

	env := map[string]string{}

	if app.Name != "" {
		env["PACK_APP_NAME"] = app.Name
	}

	// TODO: reconsider memory and disk limits during staging
	if app.Memory != "" {
		memory, err := toMegabytes(app.Memory)
		if err != nil {
			return nil, err
		}
		env["PACK_APP_MEM"] = fmt.Sprintf("%d", memory)
	}
	if app.DiskQuota != "" {
		disk, err := toMegabytes(app.DiskQuota)
		if err != nil {
			return nil, err
		}
		env["PACK_APP_DISK"] = fmt.Sprintf("%d", disk)
	}

	// TODO: remove credentials key
	if app.Services != nil {
		vcapServices, err := json.Marshal(app.Services)
		if err != nil {
			return nil, err
		}
		env["VCAP_SERVICES"] = string(vcapServices)
	}

	return &engine.ContainerConfig{
		Name:       app.Name + "-staging",
		Hostname:   app.Name,
		Env:        mapToEnv(mergeMaps(env, app.StagingEnv, app.Env)),
		Image:      stack,
		WorkingDir: "/tmp/app",
		Cmd: []string{
			"-skipDetect=" + strconv.FormatBool(!detect),
			"-buildpackOrder", strings.Join(buildpacks, ","),
		},
	}, nil
}

func streamOut(contr engine.Container, out io.Writer, path string) error {
	stream, err := contr.StreamFileFrom(path)
	if err != nil {
		return err
	}
	return stream.Out(out)
}
