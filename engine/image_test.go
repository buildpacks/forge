package engine_test

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	"github.com/docker/docker/api/types/strslice"
	docker "github.com/docker/docker/client"
	gouuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/sclevine/forge/engine"
)

var _ = Describe("Image", func() {
	var image *Image

	BeforeEach(func() {
		image = &Image{Docker: client}
	})

	Describe("#Build", func() {
		var tag string

		BeforeEach(func() {
			uuid, err := gouuid.NewV4()
			Expect(err).NotTo(HaveOccurred())
			tag = fmt.Sprintf("some-image-%s", uuid)
		})

		AfterEach(func() {
			clearImage(tag)
		})

		It("should build a Dockerfile and tag the resulting image", func() {
			dockerfile := bytes.NewBufferString(`
				FROM sclevine/test
				RUN echo some-data > /some-path
			`)
			dockerfileStream := NewStream(ioutil.NopCloser(dockerfile), int64(dockerfile.Len()))

			progress := image.Build(tag, dockerfileStream)
			naCount := 0
			for p := range progress {
				status, err := p.Status()
				Expect(err).NotTo(HaveOccurred())
				if status == "N/A" {
					naCount++
				} else {
					Expect(status).To(HaveSuffix("MB"))
				}
			}
			Expect(naCount).To(BeNumerically(">", 0))
			Expect(naCount).To(BeNumerically("<", 20))

			ctx := context.Background()
			info, _, err := client.ImageInspectWithRaw(ctx, tag)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.RepoTags[0]).To(Equal(tag + ":latest"))

			info.Config.Image = tag + ":latest"
			info.Config.Entrypoint = strslice.StrSlice{"bash"}
			contr, err := NewContainer(client, "some-name", info.Config, nil)
			Expect(err).NotTo(HaveOccurred())
			defer contr.Close()

			outStream, err := contr.StreamFileFrom("/some-path")
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(outStream)).To(Equal([]byte("some-data\n")))
		})

		It("should send an error when the Dockerfile cannot be tarred", func() {
			dockerfile := bytes.NewBufferString(`
				FROM sclevine/test
				RUN echo some-data > /some-path
			`)
			dockerfileStream := NewStream(ioutil.NopCloser(dockerfile), int64(dockerfile.Len())+100)

			progress := image.Build(tag, dockerfileStream)
			var err error
			for p := range progress {
				if _, pErr := p.Status(); pErr != nil {
					err = pErr
				}
			}
			Expect(err).To(MatchError("EOF"))

			ctx := context.Background()
			_, _, err = client.ImageInspectWithRaw(ctx, tag)
			Expect(err).To(MatchError("Error: No such image: " + tag))
		})

		It("should send an error when the image build request is invalid", func() {
			dockerfile := bytes.NewBufferString(`
				SOME BAD DOCKERFILE
			`)
			dockerfileStream := NewStream(ioutil.NopCloser(dockerfile), int64(dockerfile.Len()))

			progress := image.Build(tag, dockerfileStream)
			var err error
			for p := range progress {
				if _, pErr := p.Status(); pErr != nil {
					err = pErr
				}
			}
			Expect(err).To(MatchError(HaveSuffix("SOME")))

			ctx := context.Background()
			_, _, err = client.ImageInspectWithRaw(ctx, tag)
			Expect(err).To(MatchError("Error: No such image: " + tag))
		})

		It("should send an error when an error occurs during the image build", func() {
			dockerfile := bytes.NewBufferString(`
				FROM sclevine/test
				RUN false
			`)
			dockerfileStream := NewStream(ioutil.NopCloser(dockerfile), int64(dockerfile.Len()))

			progress := image.Build(tag, dockerfileStream)
			var err error
			for p := range progress {
				if _, err = p.Status(); err != nil {
					break
				}
			}
			Expect(err).To(MatchError(ContainSubstring("non-zero code")))
			Expect(progress).To(BeClosed())

			ctx := context.Background()
			_, _, err = client.ImageInspectWithRaw(ctx, tag)
			Expect(err).To(MatchError("Error: No such image: " + tag))
		})
	})

	// TODO: test push/pull/delete together with random ref

	Describe("#Pull", func() {
		// TODO: consider using a new image for this test
		It("should pull a Docker image", func() {
			progress := image.Pull("sclevine/test")
			naCount := 0
			for p := range progress {
				status, err := p.Status()
				Expect(err).NotTo(HaveOccurred())
				if status == "N/A" {
					naCount++
				} else {
					Expect(status).To(HaveSuffix("MB"))
				}
			}
			Expect(naCount).To(BeNumerically(">", 0))
			Expect(naCount).To(BeNumerically("<", 20))

			ctx := context.Background()
			info, _, err := client.ImageInspectWithRaw(ctx, "sclevine/test")
			Expect(err).NotTo(HaveOccurred())
			Expect(info.RepoTags[0]).To(Equal("sclevine/test:latest"))

			info.Config.Image = "sclevine/test:latest"
			info.Config.Entrypoint = strslice.StrSlice{"sh"}
			contr, err := NewContainer(client, "some-name", info.Config, nil)
			Expect(err).NotTo(HaveOccurred())
			defer contr.Close()

			outStream, err := contr.StreamFileFrom("/testfile")
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(outStream)).To(Equal([]byte("test-data\n")))
		})

		It("should send an error when the image pull request is invalid", func() {
			progress := image.Pull("-----")

			var progressErr Progress
			Expect(progress).To(Receive(&progressErr))
			_, err := progressErr.Status()
			Expect(err).To(MatchError(HaveSuffix("invalid reference format")))
			Expect(progress).To(BeClosed())
		})

		It("should send an error when an error occurs during the image build", func() {
			progress := image.Pull("sclevine/bad-test")
			var err error
			for p := range progress {
				if _, err = p.Status(); err != nil {
					break
				}
			}
			Expect(err).To(MatchError(ContainSubstring("sclevine/bad-test")))
			Expect(progress).To(BeClosed())
		})
	})

	Describe("#Push", func() {
		// TODO: setup test registry
	})

	Describe("#Delete", func() {
		It("should delete a Docker image", func() {
			uuid, err := gouuid.NewV4()
			Expect(err).NotTo(HaveOccurred())
			tag := fmt.Sprintf("some-image-%s", uuid)

			dockerfile := bytes.NewBufferString("FROM sclevine/test")
			dockerfileStream := NewStream(ioutil.NopCloser(dockerfile), int64(dockerfile.Len()))

			progress := image.Build(tag, dockerfileStream)
			for p := range progress {
				_, err := p.Status()
				Expect(err).NotTo(HaveOccurred())
			}
			defer clearImage(tag)

			Expect(image.Delete(tag)).To(Succeed())

			ctx := context.Background()
			_, _, err = client.ImageInspectWithRaw(ctx, tag)
			Expect(docker.IsErrNotFound(err)).To(BeTrue())
		})

		It("should return an error when deleting fails", func() {
			err := image.Delete("-----")
			Expect(err).To(MatchError(HaveSuffix("invalid reference format")))
		})
	})
})
