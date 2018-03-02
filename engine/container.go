package engine

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	gopath "path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	docker "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	gouuid "github.com/nu7hatch/gouuid"
)

type Container struct {
	Exit   <-chan struct{}
	Check  <-chan time.Time
	docker *docker.Client
	id     string
	config *container.Config
}

type ContainerConfig struct {
	// Internal
	Hostname   string
	User       string
	Image      string
	WorkingDir string
	Env        []string
	Entrypoint []string
	Cmd        []string

	// External
	Binds     []string
	HostIP    string
	HostPort  string
	Memory    int64
	DiskQuota int64

	// Healthcheck
	Test        []string
	Interval    time.Duration
	Timeout     time.Duration
	StartPeriod time.Duration
	Retries     int
}

type TTY interface {
	Run(remoteIn io.Reader, remoteOut io.WriteCloser, resize func(w, h uint16) error) error
}

func (e *Engine) NewContainer(name string, config *ContainerConfig) (*Container, error) {
	uuid, err := gouuid.NewV4()
	if err != nil {
		return nil, err
	}
	contConfig := &container.Config{
		Hostname:   config.Hostname,
		User:       config.User,
		Image:      config.Image,
		WorkingDir: config.WorkingDir,
		Env:        config.Env,
		Entrypoint: strslice.StrSlice(config.Entrypoint),
		Cmd:        strslice.StrSlice(config.Cmd),
	}
	if len(config.Test) > 0 {
		contConfig.Healthcheck = &container.HealthConfig{
			Test:        config.Test,
			Interval:    config.Interval,
			Timeout:     config.Timeout,
			StartPeriod: config.StartPeriod,
			Retries:     config.Retries,
		}
	}
	hostConfig := &container.HostConfig{
		Binds: config.Binds,
		PortBindings: nat.PortMap{
			"8080/tcp": {{
				HostIP:   config.HostIP,
				HostPort: config.HostPort,
			}},
		},
		Resources: container.Resources{
			Memory:    config.Memory,
			DiskQuota: config.DiskQuota,
		},
	}
	ctx := context.Background()
	response, err := e.docker.ContainerCreate(ctx, contConfig, hostConfig, nil, fmt.Sprintf("%s-%s", name, uuid))
	if err != nil {
		return nil, err
	}
	check := time.NewTicker(time.Second).C
	return &Container{e.Exit, check, e.docker, response.ID, contConfig}, nil
}

func (c *Container) ID() string {
	return c.id
}

func (c *Container) Close() error {
	ctx := context.Background()
	return c.docker.ContainerRemove(ctx, c.id, types.ContainerRemoveOptions{
		Force: true,
	})
}

func (c *Container) CloseAfterStream(stream *Stream) error {
	if stream == nil || stream.ReadCloser == nil {
		return c.Close()
	}
	stream.ReadCloser = &closeWrapper{
		ReadCloser: stream.ReadCloser,
		After:      c.Close,
	}
	return nil
}

func (c *Container) Background() error {
	ctx := context.Background()
	return c.docker.ContainerStart(ctx, c.id, types.ContainerStartOptions{})
}

func (c *Container) Start(logPrefix string, logs io.Writer, restart <-chan time.Time) (status int64, err error) {
	defer func() {
		if isErrCanceled(err) {
			status, err = 128, nil
		}
	}()
	done := make(chan struct{})
	defer close(done)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-done:
			cancel()
		case <-c.Exit:
			cancel()
		}
	}()
	logQueue := copyStreams(logs, logPrefix)
	defer close(logQueue)

	if err := c.docker.ContainerStart(ctx, c.id, types.ContainerStartOptions{}); err != nil {
		return 0, err
	}
	contLogs, err := c.docker.ContainerLogs(ctx, c.id, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Follow:     true,
	})
	if err != nil {
		return 0, err
	}
	logQueue <- contLogs

	if restart != nil {
		return c.restart(ctx, contLogs, logQueue, restart), nil
	}
	defer contLogs.Close()

	respC, errC := c.docker.ContainerWait(ctx, c.id, "")
	select {
	case resp := <-respC:
		if resp.Error != nil {
			return 0, errors.New(resp.Error.Message)
		}
		return resp.StatusCode, nil
	case err := <-errC:
		return 0, err
	}
}

func (c *Container) restart(ctx context.Context, contLogs io.ReadCloser, logQueue chan<- io.Reader, restart <-chan time.Time) (status int64) {
	// TODO: log on each continue

	for {
		select {
		case <-restart:
			wait := time.Second
			if err := c.docker.ContainerRestart(ctx, c.id, &wait); err != nil {
				continue
			}
			contJSON, err := c.docker.ContainerInspect(ctx, c.id)
			if err != nil {
				continue
			}
			startedAt, err := time.Parse(time.RFC3339Nano, contJSON.State.StartedAt)
			if err != nil {
				startedAt = time.Unix(0, 0)
			}
			contLogs.Close()
			contLogs, err = c.docker.ContainerLogs(ctx, c.id, types.ContainerLogsOptions{
				Timestamps: true,
				ShowStdout: true,
				ShowStderr: true,
				Follow:     true,
				Since:      startedAt.Add(-100 * time.Millisecond).Format(time.RFC3339Nano),
			})
			if err != nil {
				continue
			}
			logQueue <- contLogs
		case <-c.Exit:
			defer contLogs.Close()
			return 128
		}
	}
}

func isErrCanceled(err error) bool {
	return err == context.Canceled || (err != nil && strings.HasSuffix(err.Error(), "canceled"))
}

func copyStreams(dst io.Writer, prefix string) chan<- io.Reader {
	srcs := make(chan io.Reader)
	go func() {
		header := make([]byte, 8)
		for src := range srcs {
			for {
				if _, err := io.ReadFull(src, header); err != nil {
					break
				}
				if n, err := io.WriteString(dst, prefix); err != nil || n != len(prefix) {
					break
				}
				// TODO: bold STDERR
				if _, err := io.CopyN(dst, src, int64(binary.BigEndian.Uint32(header[4:]))); err != nil {
					break
				}
			}
		}
	}()
	return srcs
}

func (c *Container) Shell(tty TTY, shell ...string) (err error) {
	defer func() {
		if isErrCanceled(err) {
			err = nil
		}
	}()
	done := make(chan struct{})
	defer close(done)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-done:
			cancel()
		case <-c.Exit:
			cancel()
		}
	}()

	config := types.ExecConfig{
		User:         c.config.User,
		Tty:          true,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Env:          c.config.Env,
		Cmd:          shell,
	}
	idResp, err := c.docker.ContainerExecCreate(ctx, c.id, config)
	if err != nil {
		return err
	}

	attachResp, err := c.docker.ContainerExecAttach(ctx, idResp.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return err
	}

	return tty.Run(attachResp.Reader, attachResp.Conn, func(h, w uint16) error {
		return c.docker.ContainerExecResize(ctx, idResp.ID, types.ResizeOptions{Height: uint(h), Width: uint(w)})
	})
}

func (c *Container) HealthCheck() <-chan string {
	status := make(chan string)
	go func() {
		ctx := context.Background()
		for {
			select {
			case <-c.Exit:
				return
			case <-c.Check:
				contJSON, err := c.docker.ContainerInspect(ctx, c.id)
				if err != nil || contJSON.State == nil || contJSON.State.Health == nil {
					status <- types.NoHealthcheck
					continue
				}
				status <- contJSON.State.Health.Status
			}
		}
	}()
	return status
}

func (c *Container) Commit(ref string) (imageID string, err error) {
	ctx := context.Background()
	response, err := c.docker.ContainerCommit(ctx, c.id, types.ContainerCommitOptions{
		Reference: ref,
		Pause:     true,
		Config:    c.config,
	})
	return response.ID, err
}

func (c *Container) ExtractTo(tar io.Reader, path string) error {
	ctx := context.Background()
	return c.docker.CopyToContainer(ctx, c.id, path, onlyReader(tar), types.CopyToContainerOptions{})
}

func onlyReader(r io.Reader) io.Reader {
	if r == nil {
		return nil
	}
	return struct{ io.Reader }{r}
}

func (c *Container) StreamFileTo(stream Stream, path string) error {
	tar, err := tarFile(path, stream, stream.Size, 0755)
	if err != nil {
		return err
	}
	if err := c.ExtractTo(tar, "/"); err != nil {
		return err
	}
	return stream.Close()
}

func (c *Container) StreamTarTo(stream Stream, path string) error {
	if err := c.ExtractTo(stream, path); err != nil {
		return err
	}
	return stream.Close()
}

func (c *Container) StreamFileFrom(path string) (Stream, error) {
	ctx := context.Background()
	tar, stat, err := c.docker.CopyFromContainer(ctx, c.id, path)
	if err != nil {
		return Stream{}, err
	}
	reader, _, err := fileFromTar(gopath.Base(path), tar)
	if err != nil {
		tar.Close()
		return Stream{}, err
	}
	return NewStream(splitReadCloser{reader, tar}, stat.Size), nil
}

func (c *Container) StreamTarFrom(path string) (Stream, error) {
	ctx := context.Background()
	tar, stat, err := c.docker.CopyFromContainer(ctx, c.id, path+"/.")
	if err != nil {
		return Stream{}, err
	}
	return NewStream(tar, stat.Size), nil
}

func fileFromTar(name string, archive io.Reader) (file io.Reader, header *tar.Header, err error) {
	tarball := tar.NewReader(archive)
	for {
		header, err = tarball.Next()
		if err != nil {
			return nil, nil, err
		}
		if header.Name == name {
			break
		}
	}
	return tarball, header, nil
}

func (c *Container) Mkdir(path string) error {
	var (
		basename  = gopath.Base(path)
		dirname   = gopath.Dir(path)
		tarBuffer = &bytes.Buffer{}
		tarIn     = tar.NewWriter(tarBuffer)
	)

	if err := tarIn.WriteHeader(&tar.Header{
		Name:     basename,
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		return err
	}
	if err := tarIn.Close(); err != nil {
		return err
	}
	tarStream := NewStream(ioutil.NopCloser(tarBuffer), int64(tarBuffer.Len()))
	return c.StreamTarTo(tarStream, dirname)
}

type splitReadCloser struct {
	io.Reader
	io.Closer
}

type closeWrapper struct {
	io.ReadCloser
	After func() error
}

func (c *closeWrapper) Close() (err error) {
	defer func() {
		if afterErr := c.After(); err == nil {
			err = afterErr
		}
	}()
	return c.ReadCloser.Close()
}
