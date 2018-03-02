package engine_test

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/sclevine/forge/engine"
)

func TestEngine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Engine Suite")
}

var (
	engine *Engine
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

	engine, err = New()
	Expect(err).NotTo(HaveOccurred())

	client, err = docker.NewEnvClient()
	Expect(err).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
	Expect(engine.Close()).To(Succeed())
	Expect(client.Close()).To(Succeed())
}, func() {})
