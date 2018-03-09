package engine

import (
	"io"
	"time"
)

type Engine interface {
	NewContainer(config *ContainerConfig) (Container, error)
	NewImage() Image
	Close() error
}

type Container interface {
	io.Closer
	ID() string
	CloseAfterStream(stream *Stream) error
	Background() error
	Start(logPrefix string, logs io.Writer, restart <-chan time.Time) (status int64, err error)
	Shell(tty TTY, shell ...string) (err error)
	HealthCheck() <-chan string
	Commit(ref string) (imageID string, err error)
	UploadTarTo(tar io.Reader, path string) error
	StreamFileTo(stream Stream, path string) error
	StreamTarTo(stream Stream, path string) error
	StreamFileFrom(path string) (Stream, error)
	StreamTarFrom(path string) (Stream, error)
	Mkdir(path string) error
}

type Image interface {
	Build(tag string, dockerfile Stream) <-chan Progress
	Pull(ref string) <-chan Progress
	Push(ref string, creds RegistryCreds) <-chan Progress
	Delete(id string) error
}

type TTY interface {
	Run(remoteIn io.Reader, remoteOut io.WriteCloser, resize func(w, h uint16) error) error
}

type Progress interface {
	Status() (string, error)
}
