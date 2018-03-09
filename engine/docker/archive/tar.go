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
