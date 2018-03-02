package forge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	docker "github.com/docker/docker/client"

	"github.com/sclevine/forge/engine"
)

type Stager struct {
	Logs   io.Writer
	Loader Loader
	engine forgeEngine
	image  forgeImage
}

type StageConfig struct {
	AppTar        io.Reader
	Cache         ReadResetWriter
	CacheEmpty    bool
	BuildpackZips map[string]engine.Stream
	Stack         string
	AppDir        string
	ForceDetect   bool
	RSync         bool
	Color         Colorizer
	AppConfig     *AppConfig
}

type ReadResetWriter interface {
	io.ReadWriter
	Reset() error
}

func NewStager(client *docker.Client, exit <-chan struct{}) *Stager {
	return &Stager{
		Logs:   os.Stdout,
		Loader: noopLoader{},
		engine: &dockerEngine{
			Docker: client,
			Exit:   exit,
		},
		image: &engine.Image{
			docker: client,
			Exit:   exit,
		},
	}
}

func (s *Stager) Stage(config *StageConfig) (droplet engine.Stream, err error) {
	if err := s.pull(config.Stack); err != nil {
		return engine.Stream{}, err
	}

	containerConfig, err := s.buildContainerConfig(config.AppConfig, config.Stack, config.ForceDetect)
	if err != nil {
		return engine.Stream{}, err
	}
	contr, err := s.engine.NewContainer(config.AppConfig.Name+"-staging", containerConfig, nil)
	if err != nil {
		return engine.Stream{}, err
	}
	defer contr.CloseAfterStream(&droplet)
	for name, zip := range config.BuildpackZips {
		if err := contr.StreamFileTo(zip, fmt.Sprintf("/buildpacks/%s.zip", name)); err != nil {
			return engine.Stream{}, err
		}
	}

	if err := contr.ExtractTo(config.AppTar, "/tmp/app"); err != nil {
		return engine.Stream{}, err
	}
	if !config.CacheEmpty {
		if err := contr.ExtractTo(config.Cache, "/cache"); err != nil {
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
	if err := streamOut(contr, config.Cache, "/tmp/output-cache"); err != nil {
		return engine.Stream{}, err
	}

	return contr.StreamFileFrom("/tmp/droplet")
}

func (s *Stager) buildContainerConfig(config *AppConfig, stack string, forceDetect bool) (*container.Config, error) {
	var (
		buildpacks []string
		detect     bool
	)
	if config.Buildpack == "" && len(config.Buildpacks) == 0 {
		detect = true
	} else if len(config.Buildpacks) > 0 {
		buildpacks = config.Buildpacks
	} else {
		buildpacks = []string{config.Buildpack}
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

	if config.Name != "" {
		env["PACK_APP_NAME"] = config.Name
	}

	if config.Memory != "" {
		memory, err := toMegabytes(config.Memory)
		if err != nil {
			return nil, err
		}
		env["PACK_APP_MEM"] = fmt.Sprintf("%d", memory)
	}

	if config.DiskQuota != "" {
		disk, err := toMegabytes(config.DiskQuota)
		if err != nil {
			return nil, err
		}
		env["PACK_APP_DISK"] = fmt.Sprintf("%d", disk)
	}

	if config.Services != nil {
		vcapServices, err := json.Marshal(config.Services)
		if err != nil {
			return nil, err
		}
		env["VCAP_SERVICES"] = string(vcapServices)
	}

	return &container.Config{
		Hostname:   config.Name,
		Env:        mapToEnv(mergeMaps(env, config.StagingEnv, config.Env)),
		Image:      stack,
		WorkingDir: "/tmp/app",
		Cmd: strslice.StrSlice{
			"-skipDetect=" + strconv.FormatBool(!detect),
			"-buildpackOrder", strings.Join(buildpacks, ","),
		},
	}, nil
}

func streamOut(contr Container, out io.Writer, path string) error {
	stream, err := contr.StreamFileFrom(path)
	if err != nil {
		return err
	}
	return stream.Out(out)
}

func (s *Stager) pull(stack string) error {
	return s.Loader.Loading("Image", s.image.Pull(stack))
}
