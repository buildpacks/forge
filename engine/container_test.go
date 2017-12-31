package engine_test

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"testing/iotest"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	gouuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/sclevine/forge/engine"
	"github.com/sclevine/forge/testutil"
)

type testTTY func(io.Reader, io.Writer, func(h, w uint16) error) error

func (t testTTY) Run(remoteIn io.Reader, remoteOut io.Writer, resize func(h, w uint16) error) error {
	return t(remoteIn, remoteOut, resize)
}

var _ = Describe("Container", func() {
	var (
		contr       *Container
		config      *container.Config
		hostConfig  *container.HostConfig
		entrypoint  strslice.StrSlice
		healthcheck *container.HealthConfig
	)

	BeforeEach(func() {
		entrypoint = strslice.StrSlice{"bash"}
		healthcheck = nil
	})

	JustBeforeEach(func() {
		// TODO: specify user
		config = &container.Config{
			Healthcheck: healthcheck,
			Hostname:    "test-container",
			Image:       "sclevine/test",
			Env:         []string{"SOME-KEY=some-value"},
			Labels:      map[string]string{"some-label-key": "some-label-value"},
			Entrypoint:  entrypoint,
		}
		hostConfig = &container.HostConfig{
			PortBindings: nat.PortMap{
				"8080/tcp": {{HostIP: "127.0.0.1", HostPort: freePort()}},
			},
		}
		var err error
		contr, err = NewContainer(client, "some-name", config, hostConfig)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if containerFound(contr.ID()) {
			Expect(contr.Close()).To(Succeed())
			Expect(containerFound(contr.ID())).To(BeFalse())
		}
		Expect(client.Close()).To(Succeed())
	})

	Describe(".NewContainer", func() {
		It("should configure the container", func() {
			info := containerInfo(contr.ID())
			Expect(info.Name).To(HavePrefix("/some-name-"))
			Expect(info.Config.Env).To(ContainElement("SOME-KEY=some-value"))
			Expect(info.HostConfig.PortBindings).To(Equal(hostConfig.PortBindings))
		})
	})

	Describe("#Close", func() {
		It("should remove the container", func() {
			Expect(containerFound(contr.ID())).To(BeTrue())
			Expect(contr.Close()).To(Succeed())
			Expect(containerFound(contr.ID())).To(BeFalse())
		})

		It("should return an error if already closed", func() {
			Expect(contr.Close()).To(Succeed())
			Expect(contr.Close()).To(MatchError(ContainSubstring("No such container")))
		})
	})

	Describe("#CloseAfterStream", func() {
		It("should configure the provided stream to remove the container when it's closed", func() {
			closer := &closeTester{}
			stream := NewStream(closer, 100)
			Expect(contr.CloseAfterStream(&stream)).To(Succeed())

			Expect(closer.closed).To(BeFalse())
			Expect(containerFound(contr.ID())).To(BeTrue())

			Expect(stream.Close()).To(Succeed())

			Expect(closer.closed).To(BeTrue())
			Expect(containerFound(contr.ID())).To(BeFalse())
		})

		It("should return a container removal error if no other close error occurs", func() {
			Expect(contr.Close()).To(Succeed())

			closer := &closeTester{}
			stream := NewStream(closer, 100)
			Expect(contr.CloseAfterStream(&stream)).To(Succeed())

			Expect(contr.Close()).To(MatchError(ContainSubstring("No such container")))
			closer.err = errors.New("some error")
			Expect(stream.Close()).To(MatchError("some error"))
		})

		It("should close the container immediately if the stream is empty", func() {
			stream := NewStream(nil, 100)
			Expect(contr.CloseAfterStream(&stream)).To(Succeed())
			Expect(containerFound(contr.ID())).To(BeFalse())
		})
	})

	Describe("#Background", func() {
		BeforeEach(func() {
			entrypoint = strslice.StrSlice{
				"tail", "-f", "/dev/null",
			}
		})

		It("should start the container in the background", func() {
			Expect(containerRunning(contr.ID())).To(BeFalse())
			Expect(contr.Background()).To(Succeed())
			Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
		})

		It("should return an error when the container cannot be started", func() {
			Expect(contr.Close()).To(Succeed())
			err := contr.Background()
			Expect(err).To(MatchError(ContainSubstring("No such container")))
		})
	})

	Describe("#Start", func() {
		Context("when signaled to exit", func() {
			BeforeEach(func() {
				entrypoint = strslice.StrSlice{
					"sh", "-c",
					`echo some-logs-stdout && \
					 >&2 echo some-logs-stderr && \
					 sleep 60`,
				}
			})

			It("should start the container, stream logs, and return status 128", func() {
				wait := testutil.Wait(2)
				defer wait()

				exit := make(chan struct{})
				contr.Exit = exit

				logs := gbytes.NewBuffer()
				go func() {
					defer wait()
					defer GinkgoRecover()
					Expect(contr.Start("some-prefix", logs, nil)).To(Equal(int64(128)))
				}()
				Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
				Eventually(logs.Contents).Should(ContainSubstring("Z some-logs-stdout"))
				Eventually(logs.Contents).Should(ContainSubstring("Z some-logs-stderr"))
				close(exit)
			})
		})

		Context("when signaled to restart", func() {
			BeforeEach(func() {
				entrypoint = strslice.StrSlice{
					"sh", "-c",
					`echo some-logs-stdout && \
					 >&2 echo some-logs-stderr && \
					 sleep 60`,
				}
			})

			It("should restart until signaled to exit then return status 128", func() {
				wait := testutil.Wait(2)
				defer wait()

				exit := make(chan struct{})
				restart := make(chan time.Time)
				contr.Exit = exit

				logs := gbytes.NewBuffer()
				go func() {
					defer wait()
					defer GinkgoRecover()
					Expect(contr.Start("some-prefix", logs, restart)).To(Equal(int64(128)))
				}()
				Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
				Eventually(logs).Should(gbytes.Say("Z some-logs-stdout"))
				restart <- time.Time{}
				Eventually(logs, "5s").Should(gbytes.Say("Z some-logs-stdout"))
				restart <- time.Time{}
				Eventually(logs, "5s").Should(gbytes.Say("Z some-logs-stdout"))

				Consistently(logs, "2s").ShouldNot(gbytes.Say("Z some-logs-stdout"))
				close(exit)
			})
		})

		Context("when the command finishes successfully", func() {
			BeforeEach(func() {
				entrypoint = strslice.StrSlice{
					"sh", "-c",
					`echo some-logs-stdout && \
					 >&2 echo some-logs-stderr && \
					 sleep 0`,
				}
			})

			It("should start the container, stream logs, and return status 0", func() {
				logs := gbytes.NewBuffer()
				Expect(contr.Start("some-prefix", logs, nil)).To(Equal(int64(0)))
				Expect(containerRunning(contr.ID())).To(BeFalse())
				Expect(logs.Contents()).To(ContainSubstring("Z some-logs-stdout"))
				Expect(logs.Contents()).To(ContainSubstring("Z some-logs-stderr"))
			})
		})

		It("should return an error when the container cannot be started", func() {
			Expect(contr.Close()).To(Succeed())
			_, err := contr.Start("some-prefix", ioutil.Discard, nil)
			Expect(err).To(MatchError(ContainSubstring("No such container")))
		})
	})

	Describe("#Shell", func() {
		BeforeEach(func() {
			entrypoint = strslice.StrSlice{"sh", "-c", "sleep 5"}
		})

		It("should connect a local terminal to the container", func() {
			wait := testutil.Wait(2)
			defer wait()

			exit := make(chan struct{})
			contr.Exit = exit

			go func() {
				defer wait()
				defer GinkgoRecover()
				Expect(contr.Start("some-prefix", ioutil.Discard, nil)).To(Equal(int64(128)))
			}()

			Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
			tty := testTTY(func(in io.Reader, out io.Writer, resize func(h, w uint16) error) error {
				inBuf := &bytes.Buffer{}
				go io.Copy(inBuf, in)
				Expect(fmt.Fprint(out, "cat /testfile\n")).To(Equal(14))
				Eventually(inBuf.String).Should(ContainSubstring("cat /testfile\r\ntest-data"))
				Expect(resize(40, 50)).To(Succeed())
				Expect(fmt.Fprint(out, "stty size\n")).To(Equal(10))
				Eventually(inBuf.String).Should(ContainSubstring("40 50\r\n"))
				Expect(resize(60, 70)).To(Succeed())
				Expect(fmt.Fprint(out, "stty size\n")).To(Equal(10))
				Eventually(inBuf.String).Should(ContainSubstring("60 70\r\n"))
				Expect(fmt.Fprint(out, "exit\n")).To(Equal(5))
				Eventually(func() error { return resize(80, 90) }).Should(MatchError(ContainSubstring("process not found")))
				return nil
			})
			Expect(contr.Shell(tty, "sh")).To(Succeed())
			close(exit)
		})

		It("should not interfere with container removal when running", func() {
			wait := testutil.Wait(2)
			defer wait()

			exit := make(chan struct{})
			contr.Exit = exit

			go func() {
				defer wait()
				defer GinkgoRecover()
				Expect(contr.Start("some-prefix", ioutil.Discard, nil)).To(Equal(int64(128)))
				Expect(contr.Close()).To(Succeed())
			}()
			Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
			tty := testTTY(func(in io.Reader, out io.Writer, resize func(h, w uint16) error) error {
				return nil
			})
			Expect(contr.Shell(tty, "sh")).To(Succeed())
			close(exit)
			Eventually(try(containerRunning, contr.ID())).Should(BeFalse())
		})

		It("should stop when requested", func() {
			wait := testutil.Wait(2)
			defer wait()

			exit := make(chan struct{})
			contr.Exit = exit

			go func() {
				defer wait()
				defer GinkgoRecover()
				Expect(contr.Start("some-prefix", ioutil.Discard, nil)).To(Equal(int64(128)))
			}()
			Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
			tty := testTTY(func(in io.Reader, out io.Writer, resize func(h, w uint16) error) error {
				close(exit)
				Eventually(func() error { return resize(40, 80) }).Should(MatchError(ContainSubstring("canceled")))
				return nil
			})
			Expect(contr.Shell(tty, "sh")).To(Succeed())
		})

		It("should return an error when the shell cannot be executed", func() {
			Expect(contr.Close()).To(Succeed())
			tty := testTTY(func(in io.Reader, out io.Writer, resize func(w, h uint16) error) error {
				return nil
			})
			err := contr.Shell(tty, "sh")
			Expect(err).To(MatchError(ContainSubstring("No such container")))
		})

		It("should return an error when the TTY fails", func() {
			wait := testutil.Wait(2)
			defer wait()

			exit := make(chan struct{})
			contr.Exit = exit

			go func() {
				defer wait()
				defer GinkgoRecover()
				Expect(contr.Start("some-prefix", ioutil.Discard, nil)).To(Equal(int64(128)))
			}()
			Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
			tty := testTTY(func(in io.Reader, out io.Writer, resize func(h, w uint16) error) error {
				return errors.New("some error")
			})
			err := contr.Shell(tty, "sh")
			Expect(err).To(MatchError("some error"))

			close(exit)
		})
	})

	Describe("#HealthCheck", func() {
		Context("when the container reaches a healthy state", func() {
			BeforeEach(func() {
				entrypoint = strslice.StrSlice{
					"tail", "-f", "/dev/null",
				}
				healthcheck = &container.HealthConfig{
					Test:     []string{"CMD", "test", "-f", "/tmp/healthy"},
					Interval: 100 * time.Millisecond,
					Retries:  100,
				}
			})

			It("should report the container health", func() {
				exit, interval := make(chan struct{}), make(chan time.Time, 1)
				contr.Exit, contr.CheckInterval = exit, interval

				check := contr.HealthCheck()
				Expect(changesStatus(interval, check, "none")).To(BeTrue())

				Expect(contr.Background()).To(Succeed())
				Expect(changesStatus(interval, check, "starting")).To(BeTrue())

				empty := NewStream(ioutil.NopCloser(bytes.NewBufferString("\n")), 1)
				Expect(contr.StreamFileTo(empty, "/tmp/healthy")).To(Succeed())
				Expect(changesStatus(interval, check, "healthy")).To(BeTrue())

				exit <- struct{}{}
				interval <- time.Time{}
				Consistently(check).ShouldNot(Receive())
			})
		})
	})

	Describe("#Commit", func() {
		It("should create an image using the state of the container", func() {
			ctx := context.Background()

			inBuffer := bytes.NewBufferString("some-data")
			inStream := NewStream(ioutil.NopCloser(inBuffer), int64(inBuffer.Len()))
			Expect(contr.StreamFileTo(inStream, "/some-path")).To(Succeed())

			uuid, err := gouuid.NewV4()
			Expect(err).NotTo(HaveOccurred())
			ref := fmt.Sprintf("some-ref-%s", uuid)
			id, err := contr.Commit(ref)
			Expect(err).NotTo(HaveOccurred())
			defer client.ImageRemove(ctx, id, types.ImageRemoveOptions{
				Force:         true,
				PruneChildren: true,
			})

			info, _, err := client.ImageInspectWithRaw(ctx, id)
			Expect(err).NotTo(HaveOccurred())
			info.Config.Env = scrubEnv(info.Config.Env)
			Expect(info.Config).To(Equal(config))
			Expect(info.RepoTags[0]).To(Equal(ref + ":latest"))

			config.Image = ref + ":latest"
			contr2, err := NewContainer(client, "some-name", config, nil)
			Expect(err).NotTo(HaveOccurred())
			defer contr2.Close()

			outStream, err := contr2.StreamFileFrom("/some-path")
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(outStream)).To(Equal([]byte("some-data")))
			Expect(outStream.Size).To(Equal(inStream.Size))
		})

		It("should return an error if committing fails", func() {
			_, err := contr.Commit("$%^some-ref")
			Expect(err).To(MatchError("invalid reference format"))
		})
	})

	Describe("#ExtractTo", func() {
		It("should copy a tarball into and out of the container and not close the input", func() {
			tarBuffer := &bytes.Buffer{}
			tarIn := tar.NewWriter(tarBuffer)
			Expect(tarIn.WriteHeader(&tar.Header{Name: "some-file-1", Size: 11, Mode: 0755})).To(Succeed())
			Expect(tarIn.Write([]byte("some-data-1"))).To(Equal(11))
			Expect(tarIn.WriteHeader(&tar.Header{Name: "some-file-2", Size: 12, Mode: 0600})).To(Succeed())
			Expect(tarIn.Write([]byte("some-data-10"))).To(Equal(12))
			Expect(tarIn.Close()).To(Succeed())

			tarCloser := &closeTester{Reader: tarBuffer}
			Expect(contr.ExtractTo(tarCloser, "/root")).To(Succeed())
			Expect(tarCloser.closed).To(BeFalse())

			tarResult, err := contr.StreamTarFrom("/root")
			Expect(err).NotTo(HaveOccurred())
			defer tarResult.Close()
			tarOut := tar.NewReader(tarResult)

			header1, err := tarOut.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(header1.Name).To(Equal("./"))

			header2, err := tarOut.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(header2.Name).To(Equal("./some-file-1"))
			Expect(header2.Size).To(Equal(int64(11)))
			Expect(header2.Mode).To(Equal(int64(0100755)))
			Expect(ioutil.ReadAll(tarOut)).To(Equal([]byte("some-data-1")))

			header3, err := tarOut.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(header3.Name).To(Equal("./some-file-2"))
			Expect(header3.Size).To(Equal(int64(12)))
			Expect(header3.Mode).To(Equal(int64(0100600)))
			Expect(ioutil.ReadAll(tarOut)).To(Equal([]byte("some-data-10")))
		})

		It("should return an error if copying in fails", func() {
			err := contr.ExtractTo(nil, "/some-bad-path")
			Expect(err).To(MatchError(ContainSubstring("some-bad-path")))
		})
	})

	Describe("#StreamTarTo / #StreamTarFrom", func() {
		It("should copy a tarball into and out of the container and not close the input", func() {
			tarBuffer := &bytes.Buffer{}
			tarIn := tar.NewWriter(tarBuffer)
			Expect(tarIn.WriteHeader(&tar.Header{Name: "some-file-1", Size: 11, Mode: 0755})).To(Succeed())
			Expect(tarIn.Write([]byte("some-data-1"))).To(Equal(11))
			Expect(tarIn.WriteHeader(&tar.Header{Name: "some-file-2", Size: 12, Mode: 0600})).To(Succeed())
			Expect(tarIn.Write([]byte("some-data-10"))).To(Equal(12))
			Expect(tarIn.Close()).To(Succeed())

			tarCloser := &closeTester{Reader: tarBuffer}
			tarStream := NewStream(tarCloser, int64(tarBuffer.Len()))
			Expect(contr.StreamTarTo(tarStream, "/root")).To(Succeed())
			Expect(tarCloser.closed).To(BeTrue())

			tarResult, err := contr.StreamTarFrom("/root")
			Expect(err).NotTo(HaveOccurred())
			defer tarResult.Close()
			tarOut := tar.NewReader(tarResult)

			header1, err := tarOut.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(header1.Name).To(Equal("./"))

			header2, err := tarOut.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(header2.Name).To(Equal("./some-file-1"))
			Expect(header2.Size).To(Equal(int64(11)))
			Expect(header2.Mode).To(Equal(int64(0100755)))
			Expect(ioutil.ReadAll(tarOut)).To(Equal([]byte("some-data-1")))

			header3, err := tarOut.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(header3.Name).To(Equal("./some-file-2"))
			Expect(header3.Size).To(Equal(int64(12)))
			Expect(header3.Mode).To(Equal(int64(0100600)))
			Expect(ioutil.ReadAll(tarOut)).To(Equal([]byte("some-data-10")))

			_, err = tarOut.Next()
			Expect(err).To(Equal(io.EOF))

			Expect(tarResult.Close()).To(Succeed())

			tarResult2, err := contr.StreamTarFrom("/root/")
			Expect(err).NotTo(HaveOccurred())
			defer tarResult2.Close()
			headerDir, err := tar.NewReader(tarResult2).Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(headerDir.Name).To(Equal("./"))
		})

		It("should return an error if copying out fails", func() {
			_, err := contr.StreamTarFrom("/some-bad-path")
			Expect(err).To(MatchError(ContainSubstring("some-bad-path")))
		})

		It("should return an error if copying in fails", func() {
			errReader := ioutil.NopCloser(iotest.DataErrReader(&bytes.Buffer{}))
			err := contr.StreamTarTo(NewStream(errReader, 0), "/some-bad-path")
			Expect(err).To(MatchError(ContainSubstring("some-bad-path")))
		})
	})

	Describe("#StreamFileTo / #StreamFileFrom", func() {
		It("should copy the stream into the container and close it", func() {
			inBuffer := bytes.NewBufferString("some-data")
			inCloseTester := &closeTester{Reader: inBuffer}
			inStream := NewStream(inCloseTester, int64(inBuffer.Len()))
			Expect(contr.StreamFileTo(inStream, "/some-path/some-file")).To(Succeed())

			Expect(inCloseTester.closed).To(BeTrue())

			outStream, err := contr.StreamFileFrom("/some-path/some-file")
			Expect(err).NotTo(HaveOccurred())
			defer outStream.Close()
			Expect(ioutil.ReadAll(outStream)).To(Equal([]byte("some-data")))
			Expect(outStream.Size).To(Equal(inStream.Size))
			Expect(outStream.Close()).To(Succeed())
			// TODO: test closing of tar
		})

		It("should return an error if tarring fails", func() {
			inBuffer := bytes.NewBufferString("some-data")
			inStream := NewStream(&closeTester{Reader: inBuffer}, 100)
			err := contr.StreamFileTo(inStream, "/some-path/some-file")
			Expect(err).To(MatchError("EOF"))
		})

		It("should return an error if copy to fails", func() {
			inBuffer := bytes.NewBufferString("some-data")
			inStream := NewStream(&closeTester{Reader: inBuffer}, int64(inBuffer.Len()))
			err := contr.StreamFileTo(inStream, "/")
			Expect(err).To(MatchError(ContainSubstring("cannot overwrite")))
		})

		It("should return an error if closing fails", func() {
			inBuffer := bytes.NewBufferString("some-data")
			inCloseTester := &closeTester{Reader: inBuffer, err: errors.New("some error")}
			inStream := NewStream(inCloseTester, int64(inBuffer.Len()))
			err := contr.StreamFileTo(inStream, "/some-path/some-file")
			Expect(err).To(MatchError("some error"))
		})

		It("should return an error if copying from fails", func() {
			_, err := contr.StreamFileFrom("/some-bad-path")
			Expect(err).To(MatchError(ContainSubstring("some-bad-path")))
		})

		It("should return an error if untarring fails", func() {
			_, err := contr.StreamFileFrom("/root")
			Expect(err).To(MatchError("EOF"))
			// TODO: test closing of tar
		})
	})

	Describe("#Mkdir", func() {
		It("should create a directory in the container", func() {
			Expect(contr.Mkdir("/root/some-dir")).To(Succeed())

			tarResult, err := contr.StreamTarFrom("/root")
			Expect(err).NotTo(HaveOccurred())
			defer tarResult.Close()
			tarOut := tar.NewReader(tarResult)

			header1, err := tarOut.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(header1.Name).To(Equal("./"))

			header2, err := tarOut.Next()
			Expect(err).NotTo(HaveOccurred())
			Expect(header2.Name).To(Equal("./some-dir/"))
			Expect(header2.Mode).To(Equal(int64(040755)))
			Expect(header2.Typeflag).To(Equal(uint8(tar.TypeDir)))

			_, err = tarOut.Next()
			Expect(err).To(Equal(io.EOF))

			Expect(tarResult.Close()).To(Succeed())
		})

		It("should return an error if creating the directory fails", func() {
			err := contr.Mkdir("/some-bad-path/some-dir")
			Expect(err).To(MatchError(ContainSubstring("some-bad-path")))
		})
	})
})
