package v3

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/buildpack/packs"
	"github.com/buildpack/packs/img"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func readImage(repoName string, useDaemon bool) (v1.Image, error) {
	repoStore, err := repoStore(repoName, useDaemon)
	if err != nil {
		return nil, err
	}

	origImage, err := repoStore.Image()
	if err != nil {
		// Assume error is due to non-existent image
		return nil, nil
	}
	if _, err := origImage.RawManifest(); err != nil {
		// Assume error is due to non-existent image
		// This is necessary for registries
		return nil, nil
	}

	return origImage, nil
}

func repoStore(repoName string, useDaemon bool) (img.Store, error) {
	newRepoStore := img.NewRegistry
	if useDaemon {
		newRepoStore = img.NewDaemon
	}
	repoStore, err := newRepoStore(repoName)
	if err != nil {
		return nil, packs.FailErr(err, "access", repoName)
	}
	return repoStore, nil
}

func untarReader(r io.Reader, dir string) error {
	tr := tar.NewReader(r)
	for {
		f, err := tr.Next()
		if err == io.EOF {
			fmt.Println("    EOF")
			break
		}
		if err != nil {
			fmt.Println("    tar error:", err)
			return fmt.Errorf("tar error: %v", err)
		}
		abs := filepath.Join(dir, f.Name)

		fi := f.FileInfo()
		mode := fi.Mode()
		switch {
		case mode.IsRegular():
			if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
				return err
			}
			w, err := os.OpenFile(abs, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				return fmt.Errorf("error writing to %s: %v", abs, err)
			}
			if err := w.Close(); err != nil {
				return err
			}
		case mode.IsDir():
			if err := os.MkdirAll(abs, 0755); err != nil {
				return err
			}
		case fi.Mode()&os.ModeSymlink != 0:
			if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
				return err
			}
			if err := os.Symlink(f.Linkname, abs); err != nil {
				return err
			}
		default:
			fmt.Println("tar unsupported:", f.Name, mode)
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", f.Name, mode)
		}
	}
	fmt.Println("    TAR DONE")
	return nil
}
