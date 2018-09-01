package docker

import (
	"context"
	"fmt"
	"io"
	"os"

	eng "github.com/buildpack/forge/engine"
	docker "github.com/docker/docker/client"
)

type volume struct {
	docker    *docker.Client
	engine    eng.Engine
	name      string
	mountPath string
	image     string
}

func (e *engine) NewVolume(name, mountPath, image string) eng.Volume {
	return &volume{e.docker, e, name, mountPath, image}
}

func (v *volume) Close() error {
	ctx := context.Background()
	return v.docker.VolumeRemove(ctx, v.name, true)
}

func (v *volume) Upload(tr io.Reader) error {
	cont, err := v.container()
	if err != nil {
		return err
	}
	defer cont.Close()
	if err := cont.UploadTarTo(tr, "/"); err != nil {
		return err
	}
	if exitStatus, err := cont.Start("", os.Stdout, nil); err != nil {
		return err
	} else if exitStatus != 0 {
		return fmt.Errorf("upload failed with: %d", exitStatus)
	}
	return nil
}

func (v *volume) Export(path string) (io.ReadCloser, error) {
	cont, err := v.container()
	if err != nil {
		return nil, err
	}
	defer cont.Close()

	return cont.StreamTarFrom(path)
}

func (v *volume) String() string {
	return v.name + ":" + v.mountPath
}

func (v *volume) container() (eng.Container, error) {
	return v.engine.NewContainer(&eng.ContainerConfig{
		Name:  "forge-volume",
		Image: v.image,
		Binds: []string{
			v.name + ":" + v.mountPath,
		},
		// TODO below is very problematic
		Entrypoint: []string{},
		Cmd:        []string{"chown", "-R", "packs", v.mountPath},
		User:       "root",
	})
}
