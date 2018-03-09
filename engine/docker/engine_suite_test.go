package docker_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	eng "github.com/sclevine/forge/engine"
	. "github.com/sclevine/forge/engine/docker"
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

	return nil
}, func(_ []byte) {
	var err error

	engine, err = New(nil)
	Expect(err).NotTo(HaveOccurred())

	client, err = docker.NewEnvClient()
	Expect(err).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
	Expect(engine.Close()).To(Succeed())
	Expect(client.Close()).To(Succeed())
}, func() {})
