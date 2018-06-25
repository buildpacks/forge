package forge_test

import (
	"sort"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/sclevine/forge"
	"github.com/sclevine/forge/engine"
	"github.com/sclevine/forge/mocks"
)

var _ = Describe("Exporter", func() {
	var (
		exporter      *Exporter
		mockCtrl      *gomock.Controller
		mockEngine    *mocks.MockEngine
		mockContainer *mocks.MockContainer
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockEngine = mocks.NewMockEngine(mockCtrl)
		mockContainer = mocks.NewMockContainer(mockCtrl)

		exporter = NewExporter(mockEngine)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("#Export", func() {
		It("should load the provided droplet into a Docker image with the lifecycle", func() {
			progress := make(chan engine.Progress, 1)
			progress <- mockProgress{Value: "some-progress"}
			close(progress)
			config := &ExportConfig{
				Droplet:    engine.NewStream(mockReadCloser{Value: "some-droplet"}, 100),
				Stack:      "some-stack",
				Ref:        "some-ref",
				OutputDir:  "/home/vcap",
				WorkingDir: "/home/vcap/app",
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
			}
			mockEngine.EXPECT().NewContainer(gomock.Any()).Do(func(config *engine.ContainerConfig) {
				Expect(config.Name).To(Equal("some-name"))
				Expect(config.Hostname).To(Equal("some-name"))
				sort.Strings(config.Env)
				Expect(config.Env).To(Equal([]string{
					"PACK_APP_NAME=some-name",
					"TEST_ENV_KEY=test-env-value",
					"TEST_RUNNING_ENV_KEY=test-running-env-value",
				}))
				Expect(config.Image).To(Equal("some-stack"))
				Expect(config.WorkingDir).To(Equal("/home/vcap/app"))
				Expect(config.Entrypoint).To(Equal([]string{"/packs/launcher", "some-command"}))
			}).Return(mockContainer, nil)
			gomock.InOrder(
				mockContainer.EXPECT().StreamTarTo(config.Droplet, "/home/vcap"),
				mockContainer.EXPECT().Commit("some-ref").Return("some-image-id", nil),
				mockContainer.EXPECT().Close(),
			)
			Expect(exporter.Export(config)).To(Equal("some-image-id"))
		})

		// TODO: test with custom start command
		// TODO: test with empty app dir / without rsync
	})
})
