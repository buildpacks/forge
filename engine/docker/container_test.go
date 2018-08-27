package docker_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"testing/iotest"
	"time"

	eng "github.com/buildpack/forge/engine"
	"github.com/buildpack/forge/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/yasuyuky/jsonpath"
)

type testTTY func(io.Reader, io.WriteCloser, func(h, w uint16) error) error

func (t testTTY) Run(remoteIn io.Reader, remoteOut io.WriteCloser, resize func(h, w uint16) error) error {
	return t(remoteIn, remoteOut, resize)
}

var _ = Describe("Container", func() {
	var (
		contr      eng.Container
		config     *eng.ContainerConfig
		entrypoint []string
		healthTest []string
		exit       chan struct{}
		check      chan time.Time
	)

	BeforeEach(func() {
		entrypoint = []string{"bash"}
		healthTest = nil
		exit = nil
		check = nil
	})

	JustBeforeEach(func() {
		config = &eng.ContainerConfig{
			Name:       "some-name",
			Hostname:   "test-container",
			Image:      "sclevine/test",
			Port:       "8080",
			Env:        []string{"SOME-KEY=some-value"},
			Entrypoint: entrypoint,
			HostIP:     "127.0.0.1",
			HostPort:   freePort(),
			Test:       healthTest,
			Interval:   100 * time.Millisecond,
			Retries:    100,
			Exit:       exit,
			Check:      check,
		}
		var err error
		contr, err = engine.NewContainer(config)
		Expect(err).NotTo(HaveOccurred())
		Expect(contr.ID()).ToNot(Equal(""))
	})

	AfterEach(func() {
		if containerFound(contr.ID()) {
			Expect(contr.Close()).To(Succeed())
			Expect(containerFound(contr.ID())).To(BeFalse())
		}
	})

	// TODO: exhaustive test of options
	Describe(".NewContainer", func() {
		BeforeEach(func() {
			entrypoint = []string{"bash"}
			healthTest = []string{"echo"}
		})

		It("should configure the container", func() {
			jsonString, err := exec.Command("docker", "container", "inspect", contr.ID()).CombinedOutput()
			Expect(err).To(BeNil())
			data, err := jsonpath.DecodeString(string(jsonString))
			Expect(err).To(BeNil())
			Expect(jsonpath.GetString(data, []interface{}{0, "Name"}, "")).To(HavePrefix("/some-name-"))
			Expect(jsonpath.Get(data, []interface{}{0, "Config", "Env"}, nil)).To(ContainElement("SOME-KEY=some-value"))
			Expect(jsonpath.Get(data, []interface{}{0, "Config", "Healthcheck", "Test"}, nil)).To(Equal([]interface{}{"echo"}))
			Expect(jsonpath.Get(data, []interface{}{0, "HostConfig", "PortBindings"}, nil)).To(BeEquivalentTo(map[string]interface{}{
				"8080/tcp": []interface{}{map[string]interface{}{"HostIp": "127.0.0.1", "HostPort": config.HostPort}},
			}))
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
			stream := eng.NewStream(closer, 100)
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
			stream := eng.NewStream(closer, 100)
			Expect(contr.CloseAfterStream(&stream)).To(Succeed())

			Expect(contr.Close()).To(MatchError(ContainSubstring("No such container")))
			closer.err = errors.New("some error")
			Expect(stream.Close()).To(MatchError("some error"))
		})

		It("should close the container immediately if the stream is empty", func() {
			stream := eng.NewStream(nil, 100)
			Expect(contr.CloseAfterStream(&stream)).To(Succeed())
			Expect(containerFound(contr.ID())).To(BeFalse())
		})
	})

	Describe("#Background", func() {
		BeforeEach(func() {
			entrypoint = []string{"tail", "-f", "/dev/null"}
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
		PContext("when signaled to exit", func() {
			BeforeEach(func() {
				exit = make(chan struct{})
				entrypoint = []string{
					"sh", "-c",
					`echo some-logs-stdout && \
					 >&2 echo some-logs-stderr && \
					 sleep 60`,
				}
			})

			It("should start the container, stream logs, and return status 128", func() {
				wait := testutil.Wait(2)
				defer wait()
				defer close(exit)

				logs := gbytes.NewBuffer()
				go func() {
					defer wait()
					defer GinkgoRecover()
					Expect(contr.Start("some-prefix", logs, nil)).To(Equal(int64(128)))
				}()
				Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
				logsString := func() string { return string(logs.Contents()) }
				Eventually(logsString).Should(ContainSubstring("Z some-logs-stdout"))
				Eventually(logsString).Should(ContainSubstring("Z some-logs-stderr"))
			})
		})

		PContext("when signaled to restart", func() {
			BeforeEach(func() {
				exit = make(chan struct{})
				entrypoint = []string{
					"sh", "-c",
					`echo some-logs-stdout && \
					 >&2 echo some-logs-stderr && \
					 sleep 60`,
				}
			})

			It("should restart until signaled to exit then return status 128", func() {
				wait := testutil.Wait(2)
				defer wait()
				defer close(exit)

				restart := make(chan time.Time, 2)

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
			})
		})

		Context("when the command finishes successfully", func() {
			BeforeEach(func() {
				entrypoint = []string{
					"sh", "-c",
					`echo some-logs-stdout && \
					 >&2 echo some-logs-stderr && \
					 sleep 0`,
				}
			})

			It("should start the container, stream logs, and return status 0", func() {
				logs := gbytes.NewBuffer()
				// FIXME I changed the below prefix to include Z
				Expect(contr.Start("some-prefix Z ", logs, nil)).To(Equal(int64(0)))
				Expect(containerRunning(contr.ID())).To(BeFalse())
				Expect(string(logs.Contents())).To(ContainSubstring("Z some-logs-stdout"))
				Expect(string(logs.Contents())).To(ContainSubstring("Z some-logs-stderr"))
			})
		})

		It("should return an error when the container cannot be started", func() {
			Expect(contr.Close()).To(Succeed())
			_, err := contr.Start("some-prefix", ioutil.Discard, nil)
			Expect(err).To(MatchError(ContainSubstring("No such container")))
		})
	})

	PDescribe("#Shell", func() {
		BeforeEach(func() {
			exit = make(chan struct{})
			entrypoint = []string{"sh", "-c", "sleep 5"}
		})

		It("should connect a local terminal to the container", func() {
			wait := testutil.Wait(2)
			defer wait()
			defer close(exit)

			go func() {
				defer wait()
				defer GinkgoRecover()
				Expect(contr.Start("some-prefix", ioutil.Discard, nil)).To(Equal(int64(128)))
			}()

			Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
			tty := testTTY(func(in io.Reader, out io.WriteCloser, resize func(h, w uint16) error) error {
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
				Eventually(func() error { return resize(80, 90) }).Should(HaveOccurred())
				Expect(out.Close()).To(Succeed())
				return nil
			})
			Expect(contr.Shell(tty, "sh")).To(Succeed())
		})

		It("should not interfere with container removal when running", func() {
			wait := testutil.Wait(2)
			defer wait()

			go func() {
				defer wait()
				defer GinkgoRecover()
				Expect(contr.Start("some-prefix", ioutil.Discard, nil)).To(Equal(int64(128)))
				Expect(contr.Close()).To(Succeed())
			}()
			Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
			tty := testTTY(func(in io.Reader, out io.WriteCloser, resize func(h, w uint16) error) error {
				return nil
			})
			Expect(contr.Shell(tty, "sh")).To(Succeed())
			close(exit)
			Eventually(try(containerRunning, contr.ID())).Should(BeFalse())
		})

		It("should stop when requested", func() {
			wait := testutil.Wait(2)
			defer wait()

			go func() {
				defer wait()
				defer GinkgoRecover()
				Expect(contr.Start("some-prefix", ioutil.Discard, nil)).To(Equal(int64(128)))
			}()
			Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
			tty := testTTY(func(in io.Reader, out io.WriteCloser, resize func(h, w uint16) error) error {
				close(exit)
				Eventually(func() error { return resize(40, 80) }).Should(MatchError(ContainSubstring("canceled")))
				return nil
			})
			Expect(contr.Shell(tty, "sh")).To(Succeed())
		})

		It("should return an error when the shell cannot be executed", func() {
			Expect(contr.Close()).To(Succeed())
			tty := testTTY(func(in io.Reader, out io.WriteCloser, resize func(w, h uint16) error) error {
				return nil
			})
			err := contr.Shell(tty, "sh")
			Expect(err).To(MatchError(ContainSubstring("No such container")))
		})

		It("should return an error when the TTY fails", func() {
			wait := testutil.Wait(2)
			defer wait()
			defer close(exit)

			go func() {
				defer wait()
				defer GinkgoRecover()
				Expect(contr.Start("some-prefix", ioutil.Discard, nil)).To(Equal(int64(128)))
			}()
			Eventually(try(containerRunning, contr.ID())).Should(BeTrue())
			tty := testTTY(func(in io.Reader, out io.WriteCloser, resize func(h, w uint16) error) error {
				return errors.New("some error")
			})
			err := contr.Shell(tty, "sh")
			Expect(err).To(MatchError("some error"))
		})
	})

	// DAVE is up to here TODO
	Describe("#HealthCheck", func() {
		Context("when the container reaches a healthy state", func() {
			BeforeEach(func() {
				exit, check = make(chan struct{}), make(chan time.Time, 1)
				entrypoint = []string{"tail", "-f", "/dev/null"}
				healthTest = []string{"CMD", "test", "-f", "/tmp/healthy"}
			})

			It("should report the container health", func() {
				healthCheck := contr.HealthCheck()
				Expect(changesStatus(check, healthCheck, "none")).To(BeTrue())

				Expect(contr.Background()).To(Succeed())
				Expect(changesStatus(check, healthCheck, "starting")).To(BeTrue())

				empty := eng.NewStream(ioutil.NopCloser(bytes.NewBufferString("\n")), 1)
				Expect(contr.StreamFileTo(empty, "/tmp/healthy")).To(Succeed())
				Expect(changesStatus(check, healthCheck, "healthy")).To(BeTrue())

				exit <- struct{}{}
				check <- time.Time{}
				Consistently(healthCheck).ShouldNot(Receive())
			}, 4)
		})
	})

	Describe("#Commit", func() {
		PIt("should create an image using the state of the container", func() {
			// ctx := context.Background()
			//
			// inBuffer := bytes.NewBufferString("some-data")
			// inStream := eng.NewStream(ioutil.NopCloser(inBuffer), int64(inBuffer.Len()))
			// Expect(contr.StreamFileTo(inStream, "/some-path")).To(Succeed())
			//
			// uuid, err := gouuid.NewV4()
			// Expect(err).NotTo(HaveOccurred())
			// ref := fmt.Sprintf("some-ref-%s", uuid)
			// id, err := contr.Commit(ref)
			// Expect(err).NotTo(HaveOccurred())
			// defer client.ImageRemove(ctx, id, types.ImageRemoveOptions{
			// 	Force:         true,
			// 	PruneChildren: true,
			// })
			//
			// info, _, err := client.ImageInspectWithRaw(ctx, id)
			// Expect(err).NotTo(HaveOccurred())
			// Expect(info.Config.Hostname).To(Equal("test-container"))
			//
			// contr2, err := engine.NewContainer(&eng.ContainerConfig{
			// 	Name:       "some-name",
			// 	Image:      ref + ":latest",
			// 	Entrypoint: []string{"bash"},
			// })
			// Expect(err).NotTo(HaveOccurred())
			// defer contr2.Close()
			//
			// outStream, err := contr2.StreamFileFrom("/some-path")
			// Expect(err).NotTo(HaveOccurred())
			// Expect(ioutil.ReadAll(outStream)).To(Equal([]byte("some-data")))
			// Expect(outStream.Size).To(Equal(inStream.Size))
		})

		It("should return an error if committing fails", func() {
			_, err := contr.Commit("$%^some-ref")
			Expect(err).To(MatchError("invalid reference format"))
		})
	})

	Describe("#UploadTarTo", func() {
		It("should copy a tarball into and out of the container and not close the input", func() {
			tarBuffer := &bytes.Buffer{}
			tarIn := tar.NewWriter(tarBuffer)
			Expect(tarIn.WriteHeader(&tar.Header{Name: "some-file-1", Size: 11, Mode: 0755})).To(Succeed())
			Expect(tarIn.Write([]byte("some-data-1"))).To(Equal(11))
			Expect(tarIn.WriteHeader(&tar.Header{Name: "some-file-2", Size: 12, Mode: 0600})).To(Succeed())
			Expect(tarIn.Write([]byte("some-data-10"))).To(Equal(12))
			Expect(tarIn.Close()).To(Succeed())

			tarCloser := &closeTester{Reader: tarBuffer}
			Expect(contr.UploadTarTo(tarCloser, "/root")).To(Succeed())
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
			err := contr.UploadTarTo(nil, "/some-bad-path")
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
			tarStream := eng.NewStream(tarCloser, int64(tarBuffer.Len()))
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
			err := contr.StreamTarTo(eng.NewStream(errReader, 0), "/some-bad-path")
			Expect(err).To(MatchError(ContainSubstring("some-bad-path")))
		})
	})

	FDescribe("#StreamFileTo / #StreamFileFrom", func() {
		It("should copy the stream into the container and close it", func() {
			inBuffer := bytes.NewBufferString("some-data")
			inCloseTester := &closeTester{Reader: inBuffer}
			inStream := eng.NewStream(inCloseTester, int64(inBuffer.Len()))
			Expect(contr.StreamFileTo(inStream, "/some-path/some-file")).To(Succeed())

			Expect(inCloseTester.closed).To(BeTrue())

			outStream, err := contr.StreamFileFrom("/some-path/some-file")
			Expect(err).NotTo(HaveOccurred())
			defer outStream.Close()
			Expect(ioutil.ReadAll(outStream)).To(Equal([]byte("some-data")))
			Expect(outStream.Size).To(Equal(inStream.Size))
			Expect(outStream.Close()).To(Succeed())
			// TODO: test closing of tar
		}, 10)

		It("should return an error if tarring fails", func() {
			inBuffer := bytes.NewBufferString("some-data")
			inStream := eng.NewStream(&closeTester{Reader: inBuffer}, 100)
			err := contr.StreamFileTo(inStream, "/some-path/some-file")
			Expect(err).To(MatchError("EOF"))
		})

		It("should return an error if copy to fails", func() {
			inBuffer := bytes.NewBufferString("some-data")
			inStream := eng.NewStream(&closeTester{Reader: inBuffer}, int64(inBuffer.Len()))
			err := contr.StreamFileTo(inStream, "/")
			Expect(err).To(MatchError(ContainSubstring("write")))
		})

		It("should return an error if closing fails", func() {
			inBuffer := bytes.NewBufferString("some-data")
			inCloseTester := &closeTester{Reader: inBuffer, err: errors.New("some error")}
			inStream := eng.NewStream(inCloseTester, int64(inBuffer.Len()))
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
