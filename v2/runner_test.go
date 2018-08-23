package v2_test

import (
	"bytes"
	"sort"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/buildpack/forge/engine"
	"github.com/buildpack/forge/mocks"
	. "github.com/buildpack/forge/v2"
)

var _ = Describe("Runner", func() {
	var (
		runner        *Runner
		mockCtrl      *gomock.Controller
		mockEngine    *mocks.MockEngine
		mockContainer *mocks.MockContainer
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockEngine = mocks.NewMockEngine(mockCtrl)
		mockContainer = mocks.NewMockContainer(mockCtrl)

		runner = NewRunner(mockEngine)
		runner.Logs = bytes.NewBufferString("some-logs")
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("#Run", func() {
		It("should run the droplet in a container using the launcher", func() {
			config := &RunConfig{
				Droplet:    engine.NewStream(mockReadCloser{Value: "some-droplet"}, 100),
				Stack:      "some-stack",
				AppDir:     "some-app-dir",
				OutputDir:  "/home/vcap",
				WorkingDir: "/home/vcap/app",
				Restart:    make(<-chan time.Time),
				Color:      percentColor,
				AppConfig: &AppConfig{
					Name:      "some-name",
					Command:   "some-command",
					Memory:    "512m",
					DiskQuota: "1G",
					StagingEnv: map[string]string{
						"SOME_NA_KEY": "some-na-value",
					},
					RunningEnv: map[string]string{
						"TEST_RUNNING_ENV_KEY": "test-running-env-value",
						"TEST_ENV_KEY":         "some-overridden-value",
					},
					Env: map[string]string{
						"TEST_ENV_KEY": "test-env-value",
					},
					Services: Services{
						"some-type": {{
							Name: "some-name",
						}},
					},
				},
				NetworkConfig: &NetworkConfig{
					HostIP:      "some-ip",
					HostPort:    "400",
					ContainerID: "some-net-container",
				},
			}
			mockEngine.EXPECT().NewContainer(gomock.Any()).Do(func(config *engine.ContainerConfig) {
				Expect(config.Name).To(Equal("some-name"))
				Expect(config.Hostname).To(Equal("some-name"))
				sort.Strings(config.Env)
				Expect(config.Env).To(Equal([]string{
					"PACK_APP_DISK=1024",
					"PACK_APP_MEM=512",
					"PACK_APP_NAME=some-name",
					"TEST_ENV_KEY=test-env-value",
					"TEST_RUNNING_ENV_KEY=test-running-env-value",
					"VCAP_SERVICES=" + `{"some-type":[{"name":"some-name","label":"","tags":null,"plan":"","credentials":null,"syslog_drain_url":null,"provider":null,"volume_mounts":null}]}`,
				}))
				Expect(config.Image).To(Equal("some-stack"))
				Expect(config.WorkingDir).To(Equal("/home/vcap/app"))
				Expect(config.Entrypoint).To(HaveLen(4))
				Expect(config.Entrypoint[0]).To(Equal("/bin/bash"))
				Expect(config.Entrypoint[1]).To(Equal("-c"))
				Expect(config.Entrypoint[2]).To(ContainSubstring("launcher"))
				Expect(config.Entrypoint[3]).To(Equal("some-command"))
				Expect(config.Binds).To(Equal([]string{"some-app-dir:/tmp/local"}))
				Expect(config.NetContainer).To(Equal("some-net-container"))
				Expect(config.HostIP).To(Equal("some-ip"))
				Expect(config.HostPort).To(Equal("400"))
				Expect(config.Memory).To(Equal(int64(512 * 1024 * 1024)))
				Expect(config.DiskQuota).To(Equal(int64(1024 * 1024 * 1024)))
			}).Return(mockContainer, nil)

			gomock.InOrder(
				mockContainer.EXPECT().StreamTarTo(config.Droplet, "/home/vcap"),
				mockContainer.EXPECT().Start("[some-name] % ", runner.Logs, config.Restart).Return(int64(100), nil),
				mockContainer.EXPECT().Close(),
			)

			Expect(runner.Run(config)).To(Equal(int64(100)))
		})

		// TODO: test without bind mounts, units, shell
	})
})
