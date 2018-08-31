package docker_test

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/filters"
	volumetypes "github.com/docker/docker/api/types/volume"
	gouuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RemoveVolume", func() {
	var volName string
	BeforeEach(func() {
		uuid, err := gouuid.NewV4()
		Expect(err).ToNot(HaveOccurred())
		volName = fmt.Sprintf("myvolume-%s", uuid)
		_, err = client.VolumeCreate(context.Background(), volumetypes.VolumeCreateBody{
			Name: volName,
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("removes a volume", func() {
		Expect(volumeFound(volName)).To(BeTrue())

		Expect(engine.RemoveVolume(volName)).To(Succeed())

		Expect(volumeFound(volName)).To(BeFalse())
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
