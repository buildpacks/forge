package docker_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	eng "github.com/buildpack/forge/engine"
	. "github.com/buildpack/forge/engine/docker"
)

func TestEngine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Engine Suite")
}

var (
	engine eng.Engine
	client *docker.Client
)

var _ = SynchronizedBeforeSuite(func() []byte {
	client, err := docker.NewEnvClient()
	Expect(err).NotTo(HaveOccurred())
	defer client.Close()

	ctx := context.Background()
	body, err := client.ImagePull(ctx, "sclevine/test", types.ImagePullOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(ioutil.ReadAll(body)).NotTo(BeZero())

	dockerfile := bytes.NewBufferString(`
		FROM sclevine/test
		RUN adduser -u 1000 -s /bin/sh -S packs
		USER packs
	`)
	dockerfileStream := eng.NewStream(ioutil.NopCloser(dockerfile), int64(dockerfile.Len()))
	progress := engine.NewImage().Build("my-org/tester", dockerfileStream)
	for p := range progress {
		Expect(p).ToNot(BeNil())
	}

	return nil
}, func(_ []byte) {
	var err error

	engine, err = New(&eng.EngineConfig{})
	Expect(err).NotTo(HaveOccurred())

	client, err = docker.NewEnvClient()
	Expect(err).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
	Expect(engine.Close()).To(Succeed())
	Expect(client.Close()).To(Succeed())
}, func() {})
