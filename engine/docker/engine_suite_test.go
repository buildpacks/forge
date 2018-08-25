package docker_test

import (
	"os/exec"
	"testing"

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
)

var _ = SynchronizedBeforeSuite(func() []byte {
	Expect(exec.Command("docker", "pull", "sclevine/test").Run()).To(Succeed())

	return nil
}, func(_ []byte) {
	var err error

	engine, err = New(&eng.EngineConfig{})
	Expect(err).NotTo(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {
	Expect(engine.Close()).To(Succeed())
}, func() {})
