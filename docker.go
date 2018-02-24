package forge

import (
	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"

	"github.com/sclevine/forge/engine"
)

type dockerEngine struct {
	Docker *docker.Client
	Exit   <-chan struct{}
}

func (d *dockerEngine) NewContainer(name string, config *container.Config, hostConfig *container.HostConfig) (Container, error) {
	contr, err := engine.NewContainer(d.Docker, name, config, hostConfig)
	if err != nil {
		return nil, err
	}
	contr.Exit = d.Exit
	return contr, nil
}
