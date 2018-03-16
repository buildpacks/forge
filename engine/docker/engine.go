package docker

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"

	docker "github.com/docker/docker/client"

	eng "github.com/sclevine/forge/engine"
)

type engine struct {
	proxy  eng.ProxyConfig
	exit   <-chan struct{}
	docker *docker.Client
}

func New(config *eng.EngineConfig) (eng.Engine, error) {
	client, err := docker.NewEnvClient()
	return &engine{config.Proxy, config.Exit, client}, err
}

func (e *engine) Close() error {
	return e.docker.Close()
}

func (e *engine) proxyEnv(config *eng.ContainerConfig) []string {
	if config.SkipProxy ||
		!e.proxy.UseRemotely &&
			e.docker.DaemonHost() != docker.DefaultDockerHost {
		return nil
	}
	var env []string
	env = appendProxy(env, "http_proxy", e.proxy.HTTPProxy)
	env = appendProxy(env, "https_proxy", e.proxy.HTTPSProxy)
	env = appendProxy(env, "no_proxy", e.proxy.NoProxy)
	return env
}

func appendProxy(env []string, k, v string) []string {
	if v == "" {
		return env
	}
	return append(env,
		strings.ToLower(k)+"="+v,
		strings.ToUpper(k)+"="+v,
	)
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
