package archive

import (
	"io"

	"github.com/docker/docker/pkg/archive"
)

func Tar(dir string, files []string) (io.ReadCloser, error) {
	return archive.TarWithOptions(dir, &archive.TarOptions{
		IncludeFiles: files,
	})
}

func Copy(src, dst string) error {
	return archive.CopyResource(src, dst, false)
}