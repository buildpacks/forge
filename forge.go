package forge

import (
	"github.com/buildpack/forge/engine"
)

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
	ContainerID   string
	ContainerPort string
	HostIP        string
	HostPort      string
}

//go:generate mockgen -package mocks -destination mocks/container.go github.com/buildpack/forge/engine Container
//go:generate mockgen -package mocks -destination mocks/engine.go github.com/buildpack/forge Engine
type Engine interface {
	NewContainer(config *engine.ContainerConfig) (engine.Container, error)
}
