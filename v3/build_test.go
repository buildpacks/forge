package v3_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/ioutil"

	eng "github.com/buildpack/forge/engine"
	engdocker "github.com/buildpack/forge/engine/docker"
	. "github.com/buildpack/forge/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Builder", func() {
	var engine eng.Engine
	var builder *Builder
	BeforeEach(func() {
		var err error
		engine, err = engdocker.New(&eng.EngineConfig{})
		Expect(err).ToNot(HaveOccurred())

		dockerfile := bytes.NewBufferString(`
			FROM sclevine/test
			RUN adduser -u 1000 -s /bin/sh -S packs
			RUN echo 'hello tester' > /testfile.txt
			USER packs
		`)
		dockerfileStream := eng.NewStream(ioutil.NopCloser(dockerfile), int64(dockerfile.Len()))
		progress := engine.NewImage().Build("my-org/tester", dockerfileStream)
		for p := range progress {
			fmt.Println(p)
		}

		builder, err = NewBuilder(engine, "my-org/tester", "unique-id", "app-unique-id")
		Expect(err).ToNot(HaveOccurred())
	})
	AfterEach(func() {
		builder.Close()
		// TODO delete cache volume
	})

	Describe("#UploadToLaunch", func() {
		It("uploads files as 'packs' user", func() {
			var buf bytes.Buffer
			tw := tar.NewWriter(&buf)
			Expect(tw.WriteHeader(&tar.Header{Name: "/launch/myfile.txt", Mode: 0600, Size: 2})).To(Succeed())
			_, err := tw.Write([]byte("hi"))
			Expect(err).ToNot(HaveOccurred())
			tw.Close()
			Expect(builder.UploadToLaunch(bytes.NewReader(buf.Bytes()))).To(Succeed())

			cont, err := engine.NewContainer(&eng.ContainerConfig{
				Name:  "forge-v3-test",
				Image: "my-org/tester",
				Binds: []string{
					"pack-launch-unique-id:/launch",
				},
				Cmd: []string{"ls", "-l", "/launch/myfile.txt"},
			})
			Expect(err).ToNot(HaveOccurred())
			defer cont.Close()
			var out bytes.Buffer
			_, err = cont.Start("", &out, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(out.String()).To(ContainSubstring(" packs "))
		})
	})
})
