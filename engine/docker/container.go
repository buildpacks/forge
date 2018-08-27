package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	gopath "path"
	"strconv"
	"strings"
	"time"

	eng "github.com/buildpack/forge/engine"
	"github.com/buildpack/forge/engine/docker/httpsocket"
	"github.com/moby/moby/api/types"
	gouuid "github.com/nu7hatch/gouuid"
)

type container struct {
	exit   <-chan struct{}
	check  <-chan time.Time
	docker *httpsocket.Client
	id     string
	config map[string]interface{}
}

func (e *engine) NewContainer(config *eng.ContainerConfig) (eng.Container, error) {
	uuid, err := gouuid.NewV4()
	if err != nil {
		return nil, err
	}

	contConfig := map[string]interface{}{
		"Hostname":   config.Hostname,
		"User":       config.User,
		"Image":      config.Image,
		"WorkingDir": config.WorkingDir,
		"Env":        append(e.proxyEnv(config), config.Env...),
		"Entrypoint": config.Entrypoint,
		"Cmd":        config.Cmd,
		"HostConfig": map[string]interface{}{
			"Binds":  config.Binds,
			"Memory": int(config.Memory),
			// "DiskQuota": config.DiskQuota,
		},
	}
	if len(config.Test) > 0 {
		contConfig["Healthcheck"] = map[string]interface{}{
			"Test":        config.Test,
			"Interval":    config.Interval.Nanoseconds(),
			"Timeout":     config.Timeout.Nanoseconds(),
			"StartPeriod": config.StartPeriod.Nanoseconds(),
			"Retries":     config.Retries,
		}
	}
	if config.NetContainer != "" {
		contConfig["Hostname"] = ""
		if hc, ok := contConfig["HostConfig"].(map[string]interface{}); ok {
			hc["NetworkMode"] = "container:" + config.NetContainer
		} else {
			return nil, errors.New("could not set HostConfig NetworkMode")
		}
	} else if config.Port != "" {
		port := fmt.Sprintf("%s/tcp", config.Port)
		contConfig["ExposedPorts"] = map[string]interface{}{port: struct{}{}}
		if hc, ok := contConfig["HostConfig"].(map[string]interface{}); ok {
			hc["PortBindings"] = map[string][]map[string]interface{}{
				port: {{
					"HostIP":   config.HostIP,
					"HostPort": config.HostPort,
				}},
			}
		} else {
			return nil, errors.New("could not set HostConfig NetworkMode")
		}
	}

	check := config.Check
	if check == nil {
		check = time.NewTicker(time.Second).C
	}
	exit := config.Exit
	if exit == nil {
		exit = e.exit
	}

	response := struct {
		ID string `json:"Id"`
		// TODO do something with warnings
		Warnings string
	}{}
	if err := e.docker.Post(fmt.Sprintf("/containers/create?name=%s-%s", config.Name, uuid), contConfig, &response); err != nil {
		return &container{}, err
	}

	return &container{exit, check, e.docker, response.ID, contConfig}, nil
}

func (c *container) ID() string {
	return c.id
}

func (c *container) Close() error {
	var response map[string]string
	if err := c.docker.Delete(fmt.Sprintf("/containers/%s?force=true", c.id[:12]), &response); err != nil {
		return err
	}
	if response != nil && response["message"] != "" {
		return errors.New(response["message"])
	}

	return nil
}

func (c *container) CloseAfterStream(stream *eng.Stream) error {
	if stream == nil || stream.ReadCloser == nil {
		return c.Close()
	}
	stream.ReadCloser = &closeWrapper{
		ReadCloser: stream.ReadCloser,
		After:      c.Close,
	}
	return nil
}

func (c *container) Background() error {
	var response map[string]string
	if err := c.docker.Post(fmt.Sprintf("/containers/%s/start", c.id), nil, &response); err != nil {
		return err
	}
	if response != nil && response["message"] != "" {
		return errors.New(response["message"])
	}
	return nil
}

func (c *container) Start(logPrefix string, logs io.Writer, restart <-chan time.Time) (status int64, err error) {
	// defer func() {
	// 	if isErrCanceled(err) {
	// 		status, err = 128, nil
	// 	}
	// }()
	// done := make(chan struct{})
	// defer close(done)
	// ctx, cancel := context.WithCancel(context.Background())
	// go func() {
	// 	select {
	// 	case <-done:
	// 		cancel()
	// 	case <-c.exit:
	// 		cancel()
	// 	}
	// }()

	logQueue := copyStreams(logs, logPrefix)
	defer close(logQueue)

	if err := c.Background(); err != nil {
		return 0, err
	}

	contLogs, err := c.attachLogs()
	if err != nil {
		return 0, err
	}
	logQueue <- contLogs

	// if restart != nil {
	// 	return c.restart(ctx, contLogs, logQueue, restart), nil
	// }
	defer contLogs.Close()

	return c.wait()
}

func (c *container) attachLogs() (io.ReadCloser, error) {
	statusCode, body, err := c.docker.Do("POST", fmt.Sprintf("/containers/%s/attach?logs=true&stream=true&stdout=true&stderr=true", c.id), nil)
	if err != nil {
		return nil, err
	}
	if statusCode >= 400 {
		defer body.Close()
		var out struct{ message string }
		if err := json.NewDecoder(body).Decode(&out); err != nil {
			return nil, fmt.Errorf("status code: %d. Unkown error", statusCode)
		}
		return nil, errors.New(out.message)
	}
	return body, nil
}

func (c *container) wait() (status int64, err error) {
	response := make(map[string]interface{})
	if err := c.docker.Post(fmt.Sprintf("/containers/%s/wait", c.id), nil, &response); err != nil || response == nil {
		return 0, err
	}
	if response["StatusCode"] != nil {
		if response["Error"] != nil {
			if e, ok := response["Error"].(map[string]interface{}); ok {
				if message, ok := e["Message"].(string); ok {
					return 0, errors.New(message)
				}
			}
			return 0, fmt.Errorf("Unknown error: %#v", response)
		}
		exitCode, err := strconv.Atoi(fmt.Sprintf("%v", response["StatusCode"]))
		if err != nil {
			return 0, err
		}
		return int64(exitCode), nil
	}
	if response["message"] != nil {
		return 0, fmt.Errorf("%s", response["message"])
	}
	return 0, fmt.Errorf("Unknown error: %#v", response)
}

func (c *container) restart(ctx context.Context, contLogs io.ReadCloser, logQueue chan<- io.Reader, restart <-chan time.Time) (status int64) {
	// TODO: log on each continue

	// for {
	// 	select {
	// 	case <-restart:
	// 		wait := time.Second
	// 		if err := c.docker.ContainerRestart(ctx, c.id, &wait); err != nil {
	// 			continue
	// 		}
	// 		contJSON, err := c.docker.ContainerInspect(ctx, c.id)
	// 		if err != nil {
	// 			continue
	// 		}
	// 		startedAt, err := time.Parse(time.RFC3339Nano, contJSON.State.StartedAt)
	// 		if err != nil {
	// 			startedAt = time.Unix(0, 0)
	// 		}
	// 		contLogs.Close()
	// 		contLogs, err = c.docker.ContainerLogs(ctx, c.id, types.ContainerLogsOptions{
	// 			Timestamps: true,
	// 			ShowStdout: true,
	// 			ShowStderr: true,
	// 			Follow:     true,
	// 			Since:      startedAt.Add(-100 * time.Millisecond).Format(time.RFC3339Nano),
	// 		})
	// 		if err != nil {
	// 			continue
	// 		}
	// 		logQueue <- contLogs
	// 	case <-c.exit:
	// 		defer contLogs.Close()
	// 		return 128
	// 	}
	// }

	return 0
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

func (c *container) Shell(tty eng.TTY, shell ...string) (err error) {
	// defer func() {
	// 	if isErrCanceled(err) {
	// 		err = nil
	// 	}
	// }()
	// done := make(chan struct{})
	// defer close(done)
	// ctx, cancel := context.WithCancel(context.Background())
	// go func() {
	// 	select {
	// 	case <-done:
	// 		cancel()
	// 	case <-c.exit:
	// 		cancel()
	// 	}
	// }()
	//
	// config := types.ExecConfig{
	// 	User:         c.config.User,
	// 	Tty:          true,
	// 	AttachStdin:  true,
	// 	AttachStderr: true,
	// 	AttachStdout: true,
	// 	Env:          c.config.Env,
	// 	Cmd:          shell,
	// }
	// idResp, err := c.docker.ContainerExecCreate(ctx, c.id, config)
	// if err != nil {
	// 	return err
	// }
	//
	// attachResp, err := c.docker.ContainerExecAttach(ctx, idResp.ID, types.ExecStartCheck{Tty: true})
	// if err != nil {
	// 	return err
	// }
	//
	// return tty.Run(attachResp.Reader, attachResp.Conn, func(h, w uint16) error {
	// 	return c.docker.ContainerExecResize(ctx, idResp.ID, types.ResizeOptions{Height: uint(h), Width: uint(w)})
	// })

	return nil
}

func (c *container) HealthCheck() <-chan string {
	status := make(chan string)
	go func() {
		for {
			fmt.Println("HEALTHCHECK LOOP")
			select {
			case <-c.exit:
				return
			case <-c.check:
				var contJSON struct {
					State struct{ Health struct{ Status string } }
				}
				err := c.docker.Get(fmt.Sprintf("/containers/%s/json", c.id), &contJSON)
				fmt.Printf("HEALTHCHECK: %#v\n", contJSON)
				if err != nil || contJSON.State.Health.Status == "" {
					status <- types.NoHealthcheck
					continue
				}
				status <- contJSON.State.Health.Status
			}
		}
	}()
	return status
}

func (c *container) Commit(ref string) (imageID string, err error) {
	// ctx := context.Background()
	// response, err := c.docker.ContainerCommit(ctx, c.id, types.ContainerCommitOptions{
	// 	Reference: ref,
	// 	Pause:     true,
	// 	Config:    c.config,
	// })
	// return response.ID, err
	return "", nil
}

func (c *container) UploadTarTo(tar io.Reader, path string) error {
	statusCode, body, err := c.docker.Do("PUT", fmt.Sprintf("/containers/%s/archive?path=%s", c.id, path), tar)
	if err != nil || statusCode >= 400 {
		return fmt.Errorf("UploadTarTo(%s): %s => %d, %v, %s\n", c.id, path, statusCode, err, body)
	}

	return nil
}

func onlyReader(r io.Reader) io.Reader {
	if r == nil {
		return nil
	}
	return struct{ io.Reader }{r}
}

func (c *container) StreamFileTo(stream eng.Stream, path string) error {
	tar, err := tarFile(path, stream, stream.Size, 0755)
	if err != nil {
		return err
	}
	if err := c.UploadTarTo(tar, "/"); err != nil {
		return err
	}
	return stream.Close()
}

func (c *container) StreamTarTo(stream eng.Stream, path string) error {
	if err := c.UploadTarTo(stream, path); err != nil {
		return err
	}
	return stream.Close()
}

func (c *container) StreamFileFrom(path string) (eng.Stream, error) {
	// ctx := context.Background()
	// tar, stat, err := c.docker.CopyFromContainer(ctx, c.id, path)
	// if err != nil {
	// 	return eng.Stream{}, err
	// }
	// reader, _, err := fileFromTar(gopath.Base(path), tar)
	// if err != nil {
	// 	tar.Close()
	// 	return eng.Stream{}, err
	// }
	// return eng.NewStream(splitReadCloser{reader, tar}, stat.Size), nil

	statusCode, body, err := c.docker.Do("GET", fmt.Sprintf("/containers/%s/archive?path=%s", c.id, path), nil)
	if err != nil || statusCode >= 400 {
		if err == nil {
			defer body.Close()
		}
		return eng.Stream{}, fmt.Errorf("StreamFileFrom(%s): %s => %d, %v, %s\n", c.id, path, statusCode, err, body)
	}
	reader, hdr, err := fileFromTar(gopath.Base(path), body)
	if err != nil {
		body.Close()
		return eng.Stream{}, err
	}
	return eng.NewStream(splitReadCloser{reader, body}, hdr.Size), nil
}

func (c *container) StreamTarFrom(path string) (eng.Stream, error) {
	// ctx := context.Background()
	// tar, stat, err := c.docker.CopyFromContainer(ctx, c.id, path+"/.")
	// if err != nil {
	// 	return eng.Stream{}, err
	// }
	// return eng.NewStream(tar, stat.Size), nil
	return eng.Stream{}, nil
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

func (c *container) Mkdir(path string) error {
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
	tarStream := eng.NewStream(ioutil.NopCloser(tarBuffer), int64(tarBuffer.Len()))
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

type readCloserTimestamp struct {
	io.ReadCloser
}
