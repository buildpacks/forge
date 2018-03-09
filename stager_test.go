package forge_test

import (
	"bytes"
	"sort"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/sclevine/forge"
	"github.com/sclevine/forge/engine"
	"github.com/sclevine/forge/mocks"
)

var _ = Describe("Stager", func() {
	var (
		stager        *Stager
		mockCtrl      *gomock.Controller
		mockLoader    *mocks.MockLoader
		mockEngine    *mocks.MockEngine
		mockImage     *mocks.MockImage
		mockContainer *mocks.MockContainer
		logs          *bytes.Buffer
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockLoader = mocks.NewMockLoader()
		mockEngine = mocks.NewMockEngine(mockCtrl)
		mockImage = mocks.NewMockImage(mockCtrl)
		mockContainer = mocks.NewMockContainer(mockCtrl)
		logs = bytes.NewBufferString("some logs\n")

		stager = NewStager(mockEngine)
		stager.Logs = logs
		stager.Loader = mockLoader
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("#Stage", func() {
		It("should return a droplet of a staged app", func() {
			buildpackZipStream1 := engine.NewStream(mockReadCloser{Value: "some-buildpack-zip-1"}, 100)
			buildpackZipStream2 := engine.NewStream(mockReadCloser{Value: "some-buildpack-zip-2"}, 200)
			localCache := mocks.NewMockBuffer("some-old-cache")
			remoteCache := mocks.NewMockBuffer("some-new-cache")
			remoteCacheStream := engine.NewStream(remoteCache, int64(remoteCache.Len()))
			dropletStream := engine.NewStream(mockReadCloser{Value: "some-droplet"}, 300)

			progress := make(chan engine.Progress, 1)
			progress <- mockProgress{Value: "some-progress"}
			close(progress)

			config := &StageConfig{
				AppTar: bytes.NewBufferString("some-app-tar"),
				Cache:  localCache,
				BuildpackZips: map[string]engine.Stream{
					"some-name-one": buildpackZipStream1,
					"some-name-two": buildpackZipStream2,
				},
				Stack: "some-stack",
				Color: percentColor,
				AppConfig: &AppConfig{
					Name:      "some-name",
					Buildpack: "some-buildpack",
					Buildpacks: []string{
						"some-buildpack-one",
						"some-buildpack-two",
					},
					StagingEnv: map[string]string{
						"TEST_STAGING_ENV_KEY": "test-staging-env-value",
						"MEMORY_LIMIT":         "256m",
					},
					RunningEnv: map[string]string{
						"SOME_NA_KEY": "some-na-value",
					},
					Env: map[string]string{
						"TEST_ENV_KEY": "test-env-value",
						"MEMORY_LIMIT": "1024m",
					},
					Services: Services{
						"some-type": {{
							Name: "some-name",
						}},
					},
				},
			}
			mockEngine.EXPECT().NewImage().Return(mockImage)
			gomock.InOrder(
				mockImage.EXPECT().Pull("some-stack").Return(progress),
				mockEngine.EXPECT().NewContainer(gomock.Any()).Do(func(config *engine.ContainerConfig) {
					Expect(config.Name).To(Equal("some-name-staging"))
					Expect(config.Hostname).To(Equal("some-name"))
					sort.Strings(config.Env)
					Expect(config.Env).To(Equal([]string{
						"MEMORY_LIMIT=1024m",
						"PACK_APP_NAME=some-name",
						"TEST_ENV_KEY=test-env-value",
						"TEST_STAGING_ENV_KEY=test-staging-env-value",
						"VCAP_SERVICES=" + `{"some-type":[{"name":"some-name","label":"","tags":null,"plan":"","credentials":null,"syslog_drain_url":null,"provider":null,"volume_mounts":null}]}`,
					}))
					Expect(config.Image).To(Equal("some-stack"))
					Expect(config.WorkingDir).To(Equal("/tmp/app"))
					Expect(config.Cmd).To(Equal([]string{"-skipDetect=true", "-buildpackOrder", "some-buildpack-one,some-buildpack-two"}))
				}).Return(mockContainer, nil),
			)

			gomock.InOrder(
				mockContainer.EXPECT().Start("[some-name] % ", logs, nil).Return(int64(0), nil).
					After(mockContainer.EXPECT().StreamFileTo(buildpackZipStream1, "/buildpacks/some-name-one.zip")).
					After(mockContainer.EXPECT().StreamFileTo(buildpackZipStream2, "/buildpacks/some-name-two.zip")).
					After(mockContainer.EXPECT().UploadTarTo(config.AppTar, "/tmp/app")).
					After(mockContainer.EXPECT().UploadTarTo(localCache, "/tmp/cache")),
				mockContainer.EXPECT().StreamFileFrom("/cache/cache.tgz").Return(remoteCacheStream, nil),
				mockContainer.EXPECT().StreamFileFrom("/out/droplet.tgz").Return(dropletStream, nil),
				mockContainer.EXPECT().CloseAfterStream(&dropletStream),
			)

			Expect(stager.Stage(config)).To(Equal(dropletStream))
			Expect(localCache.Close()).To(Succeed())
			Expect(localCache.Result()).To(Equal("some-new-cache"))
			Expect(remoteCache.Result()).To(BeEmpty())
			Expect(logs.String()).To(Equal("some logs\nBuildpacks: some-buildpack-one, some-buildpack-two\n"))
			Expect(mockLoader.Progress).To(Receive(Equal(mockProgress{Value: "some-progress"})))
		})

		// TODO: test unavailable buildpack versions
		// TODO: test empty cache
		// TODO: test single-buildpack case, detection, force detection
		// TODO: test non-zero command return status
	})
})
