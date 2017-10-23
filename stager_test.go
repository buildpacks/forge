package forge_test

import (
	"bytes"
	"io/ioutil"
	"sort"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/sclevine/forge"
	"github.com/sclevine/forge/engine"
	"github.com/sclevine/forge/fixtures"
	"github.com/sclevine/forge/mocks"
	"github.com/sclevine/forge/service"
)

var _ = Describe("Stager", func() {
	var (
		stager        *Stager
		mockCtrl      *gomock.Controller
		mockLoader    *mocks.MockLoader
		mockEngine    *mocks.MockEngine
		mockImage     *mocks.MockImage
		mockVersioner *mocks.MockVersioner
		mockContainer *mocks.MockContainer
		logs          *bytes.Buffer
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockLoader = mocks.NewMockLoader()
		mockEngine = mocks.NewMockEngine(mockCtrl)
		mockImage = mocks.NewMockImage(mockCtrl)
		mockVersioner = mocks.NewMockVersioner(mockCtrl)
		mockContainer = mocks.NewMockContainer(mockCtrl)
		logs = bytes.NewBufferString("some logs\n")

		stager = &Stager{
			ImageTag: "some-tag",
			SystemBuildpacks: Buildpacks{
				{Name: "some-buildpack-name-1", URL: "some-buildpack-url-1", VersionURL: "some-buildpack-version-url-1"},
				{Name: "some-buildpack-name-2", URL: "some-buildpack-url-2", VersionURL: "some-buildpack-version-url-2"},
			},
			Logs:      logs,
			Loader:    mockLoader,
			Engine:    mockEngine,
			Image:     mockImage,
			Versioner: mockVersioner,
		}
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
				AppTar:     bytes.NewBufferString("some-app-tar"),
				Cache:      localCache,
				CacheEmpty: false,
				BuildpackZips: map[string]engine.Stream{
					"some-checksum-one": buildpackZipStream1,
					"some-checksum-two": buildpackZipStream2,
				},
				Stack:  "some-stack",
				AppDir: "some-app-dir",
				RSync:  true,
				Color:  percentColor,
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
					Services: service.Services{
						"some-type": {{
							Name: "some-name",
						}},
					},
				},
			}

			Expect(stager.SystemBuildpacks).NotTo(HaveLen(0))
			for _, buildpack := range stager.SystemBuildpacks {
				mockVersioner.EXPECT().Build(buildpack.URL, buildpack.VersionURL).Return(buildpack.Name+"-versioned-url", nil)
			}

			gomock.InOrder(
				mockImage.EXPECT().Build(gomock.Any(), gomock.Any()).Do(func(tag string, dockerfile engine.Stream) {
					Expect(tag).To(Equal("some-tag"))
					dfBytes, err := ioutil.ReadAll(dockerfile)
					Expect(err).NotTo(HaveOccurred())

					Expect(dockerfile.Size).To(Equal(int64(len(dfBytes))))
					Expect(dfBytes).To(ContainSubstring("FROM some-stack"))
					Expect(dfBytes).To(ContainSubstring(`"some-buildpack-name-1-versioned-url"`))
					Expect(dfBytes).To(ContainSubstring("/tmp/buildpacks/0d75acab3a54a7f434405f9cce289eb7"))
					Expect(dfBytes).To(ContainSubstring(`"some-buildpack-name-2-versioned-url"`))
					Expect(dfBytes).To(ContainSubstring("/tmp/buildpacks/d144777ad4a8489c942ce839db1ad73d"))
				}).Return(progress),
				mockEngine.EXPECT().NewContainer("some-name-staging", gomock.Any(), gomock.Any()).Do(func(_ string, config *container.Config, hostConfig *container.HostConfig) {
					Expect(config.Hostname).To(Equal("some-name"))
					Expect(config.User).To(Equal("root"))
					Expect(config.ExposedPorts).To(HaveLen(0))
					sort.Strings(config.Env)
					Expect(config.Env).To(Equal(fixtures.ProvidedStagingEnv("MEMORY_LIMIT=1024m")))
					Expect(config.Image).To(Equal("some-tag"))
					Expect(config.WorkingDir).To(Equal("/home/vcap"))
					Expect(config.Entrypoint).To(Equal(strslice.StrSlice{
						"/bin/bash", "-c", fixtures.StageRSyncScript(),
						"some-buildpack-one,some-buildpack-two", "true",
					}))
					Expect(hostConfig.Binds).To(Equal([]string{"some-app-dir:/tmp/local"}))
				}).Return(mockContainer, nil),
			)

			buildpackCopy1 := mockContainer.EXPECT().CopyTo(buildpackZipStream1, "/tmp/some-checksum-one.zip")
			buildpackCopy2 := mockContainer.EXPECT().CopyTo(buildpackZipStream2, "/tmp/some-checksum-two.zip")
			appExtract := mockContainer.EXPECT().ExtractTo(config.AppTar, "/tmp/app")
			cacheExtract := mockContainer.EXPECT().ExtractTo(localCache, "/tmp/cache")

			gomock.InOrder(
				mockContainer.EXPECT().Start("[some-name] % ", logs, nil).Return(int64(0), nil).
					After(buildpackCopy1).
					After(buildpackCopy2).
					After(appExtract).
					After(cacheExtract),
				mockContainer.EXPECT().CopyFrom("/tmp/output-cache").Return(remoteCacheStream, nil),
				mockContainer.EXPECT().CopyFrom("/tmp/droplet").Return(dropletStream, nil),
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
		// TODO: test single-buildpack case
		// TODO: test non-zero command return status
		// TODO: test no app dir case
		// TODO: test without rsync
	})

	Describe("#Download", func() {
		It("should return the specified file", func() {
			progress := make(chan engine.Progress, 1)
			progress <- mockProgress{Value: "some-progress"}
			close(progress)

			Expect(stager.SystemBuildpacks).NotTo(HaveLen(0))
			for _, buildpack := range stager.SystemBuildpacks {
				mockVersioner.EXPECT().Build(buildpack.URL, buildpack.VersionURL).Return(buildpack.Name+"-versioned-url", nil)
			}

			gomock.InOrder(
				mockImage.EXPECT().Build(gomock.Any(), gomock.Any()).Do(func(tag string, dockerfile engine.Stream) {
					Expect(tag).To(Equal("some-tag"))
					dfBytes, err := ioutil.ReadAll(dockerfile)
					Expect(err).NotTo(HaveOccurred())

					Expect(dockerfile.Size).To(Equal(int64(len(dfBytes))))
					Expect(dfBytes).To(ContainSubstring("some-stack"))
					Expect(dfBytes).To(ContainSubstring(`"some-buildpack-name-1-versioned-url"`))
					Expect(dfBytes).To(ContainSubstring("/tmp/buildpacks/0d75acab3a54a7f434405f9cce289eb7"))
					Expect(dfBytes).To(ContainSubstring(`"some-buildpack-name-2-versioned-url"`))
					Expect(dfBytes).To(ContainSubstring("/tmp/buildpacks/d144777ad4a8489c942ce839db1ad73d"))
				}).Return(progress),
				mockEngine.EXPECT().NewContainer("download", gomock.Any(), gomock.Any()).Do(func(_ string, config *container.Config, hostConfig *container.HostConfig) {
					Expect(config.Hostname).To(Equal("download"))
					Expect(config.User).To(Equal("root"))
					Expect(config.ExposedPorts).To(HaveLen(0))
					Expect(config.Image).To(Equal("some-tag"))
					Expect(config.Entrypoint).To(Equal(strslice.StrSlice{"read"}))
					Expect(hostConfig).To(BeNil())
				}).Return(mockContainer, nil),
			)

			stream := engine.NewStream(mockReadCloser{Value: "some-stream"}, 100)
			gomock.InOrder(
				mockContainer.EXPECT().CopyFrom("/some-path").Return(stream, nil),
				mockContainer.EXPECT().CloseAfterStream(&stream),
			)

			Expect(stager.Download("/some-path", "some-stack")).To(Equal(stream))
			Expect(mockLoader.Progress).To(Receive(Equal(mockProgress{Value: "some-progress"})))
		})
	})
})
