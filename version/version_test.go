package version_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/sclevine/forge/version"
)

var _ = Describe("Version", func() {
	var version *Version

	BeforeEach(func() {
		version = &Version{Client: &http.Client{}}
	})

	Describe("#Build", func() {
		It("should lookup the version and embed it in the template", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				if r.Method == "GET" {
					w.Write([]byte("some-version"))
				}
			}))
			out, err := version.Build("some-{{.}}-template", server.URL)
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(Equal("some-some-version-template"))
		})
	})
})
