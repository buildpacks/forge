package docker

import (
	"archive/tar"
	"bytes"
	"io"

	docker "github.com/docker/docker/client"

	eng "github.com/sclevine/forge/engine"
)

type engine struct {
	Exit   <-chan struct{}
	docker *docker.Client
}

func New() (eng.Engine, error) {
	client, err := docker.NewEnvClient()
	return &engine{docker: client}, err
}

func (e *engine) Close() error {
	return e.docker.Close()
}

func tarFile(name string, contents io.Reader, size, mode int64) (io.Reader, error) {
	tarBuffer := &bytes.Buffer{}
	tarball := tar.NewWriter(tarBuffer)
	defer tarball.Close()
	header := &tar.Header{Name: name, Size: size, Mode: mode}
	if err := tarball.WriteHeader(header); err != nil {
		return nil, err
	}
	if _, err := io.CopyN(tarball, contents, size); err != nil {
		return nil, err
	}
	return tarBuffer, nil
}
