package forge

import (
	"io"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/sclevine/forge/engine"
)

//go:generate mockgen -package mocks -destination mocks/container.go github.com/sclevine/forge Container
type Container interface {
	io.Closer
	ID() string
	CloseAfterStream(stream *engine.Stream) error
	Background() error
	Start(logPrefix string, logs io.Writer, restart <-chan time.Time) (status int64, err error)
	Shell(tty engine.TTY, shell ...string) (err error)
	HealthCheck() <-chan string
	Commit(ref string) (imageID string, err error)
	ExtractTo(tar io.Reader, path string) error
	StreamTarTo(stream engine.Stream, path string) error
	StreamFileTo(stream engine.Stream, path string) error
	StreamFileFrom(path string) (engine.Stream, error)
}

type Loader interface {
	Loading(message string, progress <-chan engine.Progress) error
}

type Colorizer func(string, ...interface{}) string

type AppConfig struct {
	Name       string            `yaml:"name"`
	Buildpack  string            `yaml:"buildpack,omitempty"`
	Buildpacks []string          `yaml:"buildpacks,omitempty"`
	Command    string            `yaml:"command,omitempty"`
	DiskQuota  string            `yaml:"disk_quota,omitempty"`
	Memory     string            `yaml:"memory,omitempty"`
	StagingEnv map[string]string `yaml:"staging_env,omitempty"`
	RunningEnv map[string]string `yaml:"running_env,omitempty"`
	Env        map[string]string `yaml:"env,omitempty"`
	Services   Services          `yaml:"services,omitempty"`
}

type NetworkConfig struct {
	ContainerID string
	HostIP      string
	HostPort    string
}

//go:generate mockgen -package mocks -destination mocks/image.go github.com/sclevine/forge/mocks Image
type forgeImage interface {
	Pull(ref string) <-chan engine.Progress
}

//go:generate mockgen -package mocks -destination mocks/engine.go github.com/sclevine/forge/mocks Engine
type forgeEngine interface {
	NewContainer(name string, config *container.Config, hostConfig *container.HostConfig) (Container, error)
}
