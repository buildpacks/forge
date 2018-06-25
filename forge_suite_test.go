package forge_test

import (
	"fmt"
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/buildpack/forge/engine"
)

func TestLocal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Forge Suite")
}

func percentColor(format string, a ...interface{}) string {
	return fmt.Sprintf(format+"%% ", a...)
}

type mockProgress struct {
	Value string
	engine.Progress
}

type mockReadCloser struct {
	Value string
	io.ReadCloser
}
