package forge_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/sclevine/forge"
	"github.com/sclevine/forge/engine"
	"github.com/sclevine/forge/mocks"
)

var _ = Describe("Forwarder", func() {
	var (
		forwarder        *Forwarder
		mockCtrl         *gomock.Controller
		mockEngine       *mocks.MockEngine
		mockNetContainer *mocks.MockContainer
		mockContainer    *mocks.MockContainer
		logs             *gbytes.Buffer
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockEngine = mocks.NewMockEngine(mockCtrl)
		mockNetContainer = mocks.NewMockContainer(mockCtrl)
		mockContainer = mocks.NewMockContainer(mockCtrl)
		logs = gbytes.NewBuffer()

		forwarder = NewForwarder(mockEngine)
		forwarder.Logs = logs
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("#Forward", func() {
		It("should configure service tunnels and general app networking", func() {
			mockHealth := make(<-chan string)
			waiter := make(chan time.Time)
			codeIdx := 0
			config := &ForwardConfig{
				AppName: "some-name",
				Stack:   "some-stack",
				Color:   percentColor,
				Details: &ForwardDetails{
					Host: "some-ssh-host",
					Port: "some-port",
					User: "some-user",
					Code: func() (string, error) {
						codeIdx++
						return fmt.Sprintf("some-code-%d", codeIdx), nil
					},
					Forwards: []Forward{
						{
							Name: "some-name",
							From: "some-from",
							To:   "some-to",
						},
						{
							Name: "some-other-name",
							From: "some-other-from",
							To:   "some-other-to",
						},
					},
				},
				HostIP:   "some-ip",
				HostPort: "400",
				Wait:     waiter,
			}
			mockEngine.EXPECT().NewContainer(gomock.Any()).Do(func(config *engine.ContainerConfig) {
				Expect(config.Name).To(Equal("network"))
				Expect(config.Hostname).To(Equal("some-name"))
				Expect(config.Image).To(Equal("some-stack"))
				Expect(config.Entrypoint).To(Equal([]string{"tail", "-f", "/dev/null"}))
				Expect(config.HostIP).To(Equal("some-ip"))
				Expect(config.HostPort).To(Equal("400"))
				Expect(config.Exit).NotTo(BeNil())
			}).Return(mockNetContainer, nil)

			mockNetContainer.EXPECT().ID().Return("some-net-container").AnyTimes()
			gomock.InOrder(
				mockNetContainer.EXPECT().Background(),
				mockEngine.EXPECT().NewContainer(gomock.Any()).Do(func(config *engine.ContainerConfig) {
					Expect(config.Name).To(Equal("service"))
					Expect(config.Image).To(Equal("some-stack"))
					Expect(config.Entrypoint).To(HaveLen(3))
					Expect(config.Entrypoint[0]).To(Equal("/bin/bash"))
					Expect(config.Entrypoint[1]).To(Equal("-c"))
					Expect(config.Entrypoint[2]).To(ContainSubstring("sshpass"))
					Expect(config.NetContainer).To(Equal("some-net-container"))
					Expect(config.Test).To(Equal([]string{"CMD", "test", "-f", "/tmp/healthy"}))
					Expect(config.Interval).To(Equal(time.Second))
					Expect(config.Retries).To(Equal(30))
					Expect(config.Exit).NotTo(BeNil())
				}).Return(mockContainer, nil),
			)

			mockContainer.EXPECT().HealthCheck().Return(mockHealth)

			health, done, id, err := forwarder.Forward(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(health).To(Equal(mockHealth))
			Expect(id).To(Equal("some-net-container"))

			gomock.InOrder(
				mockContainer.EXPECT().StreamFileTo(gomock.Any(), "/tmp/ssh-code").Do(func(stream engine.Stream, _ string) {
					defer GinkgoRecover()
					defer stream.Close()
					Expect(ioutil.ReadAll(stream)).To(Equal([]byte("some-code-1")))
				}),
				mockContainer.EXPECT().Start("[some-name tunnel] % ", gomock.Any(), nil).Do(func(_ string, output io.Writer, _ <-chan time.Time) {
					fmt.Fprint(output, "start-1")
				}).Return(int64(100), nil),
				mockContainer.EXPECT().StreamFileTo(gomock.Any(), "/tmp/ssh-code").Do(func(stream engine.Stream, _ string) {
					defer GinkgoRecover()
					defer stream.Close()
					Expect(ioutil.ReadAll(stream)).To(Equal([]byte("some-code-2")))
				}),
				mockContainer.EXPECT().Start("[some-name tunnel] % ", gomock.Any(), nil).Do(func(_ string, output io.Writer, _ <-chan time.Time) {
					fmt.Fprint(output, "start-2")
					done()
				}).Return(int64(200), nil),
				mockContainer.EXPECT().Close(),
				mockNetContainer.EXPECT().Close(),
			)

			waiter <- time.Time{}
			waiter <- time.Time{}

			Eventually(logs).Should(gbytes.Say(`start-1\[some-name tunnel\] % Exited with status: 100`))
			Eventually(logs).Should(gbytes.Say("start-2"))
			Consistently(logs).ShouldNot(gbytes.Say(`\[some-name tunnel\] % Exited with status: 200`))
		})
	})
})
