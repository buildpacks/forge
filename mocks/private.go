package mocks

import (
	"github.com/docker/docker/api/types/container"
	"github.com/sclevine/forge"
	"github.com/sclevine/forge/engine"
)

type Image interface {
	Pull(ref string) <-chan engine.Progress
	Build(tag string, dockerfile engine.Stream) <-chan engine.Progress
}

type Engine interface {
	NewContainer(name string, config *container.Config, hostConfig *container.HostConfig) (forge.Container, error)
}
