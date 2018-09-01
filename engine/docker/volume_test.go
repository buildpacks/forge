package docker_test

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"

	eng "github.com/buildpack/forge/engine"
	"github.com/docker/docker/api/types/filters"
	volumetypes "github.com/docker/docker/api/types/volume"
	gouuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("Volume", func() {
	var volume eng.Volume
	var volName string
	BeforeEach(func() {
		uuid, err := gouuid.NewV4()
		Expect(err).ToNot(HaveOccurred())
		volName = fmt.Sprintf("myvolume-%s", uuid)
		volume = engine.NewVolume(volName, "/myvol", "my-org/tester")
	})

	Describe("#Close", func() {
		BeforeEach(func() {
			_, err := client.VolumeCreate(context.Background(), volumetypes.VolumeCreateBody{
				Name: volName,
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("removes a volume", func() {
			Expect(volumeFound(volName)).To(BeTrue())

			Expect(volume.Close()).To(Succeed())

			Expect(volumeFound(volName)).To(BeFalse())
		})
	})

	Describe("#Upload", func() {
		It("uploads files as 'packs' user", func() {
			Expect(volume.Upload(simpleTarReader())).To(Succeed())

			cont, err := engine.NewContainer(&eng.ContainerConfig{
				Name:  "forge-v3-test",
				Image: "my-org/tester",
				Binds: []string{
					volName + ":/myvol",
				},
				Cmd: []string{"ls", "-l", "/myvol/myfile.txt"},
			})
			Expect(err).ToNot(HaveOccurred())
			defer cont.Close()
			var out bytes.Buffer
			_, err = cont.Start("", &out, nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(out.String()).To(ContainSubstring(" packs "))
		})
	})

	Describe("#Export", func() {
		It("streams files out", func() {
			Expect(volume.Upload(simpleTarReader())).To(Succeed())
			rc, err := volume.Export("/myvol/")
			Expect(err).ToNot(HaveOccurred())

			tr := tar.NewReader(rc)
			_, err = tr.Next()
			Expect(err).ToNot(HaveOccurred())
			hdr, err := tr.Next()
			Expect(err).ToNot(HaveOccurred())

			Expect(hdr.Name).To(Equal("./myfile.txt"))
			Expect(ioutil.ReadAll(tr)).To(Equal([]byte("hi")))
		})
	})
})

func volumeFound(name string) bool {
	volumes, err := client.VolumeList(context.Background(), filters.NewArgs())
	Expect(err).ToNot(HaveOccurred())
	for _, volume := range volumes.Volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

func simpleTarReader() io.Reader {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	Expect(tw.WriteHeader(&tar.Header{Name: "/myvol/myfile.txt", Mode: 0600, Size: 2})).To(Succeed())
	_, err := tw.Write([]byte("hi"))
	Expect(err).ToNot(HaveOccurred())
	tw.Close()
	return bytes.NewReader(buf.Bytes())
}
