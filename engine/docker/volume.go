package docker

import (
	"context"
)

func (e *engine) RemoveVolume(name string) error {
	ctx := context.Background()
	return e.docker.VolumeRemove(ctx, name, true)
}
