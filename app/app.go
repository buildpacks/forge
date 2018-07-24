package app

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"code.cloudfoundry.org/cli/cf/appfiles"

	"github.com/buildpack/forge/engine/docker/archive"
)

func Tar(path string, excludes ...string) (app io.ReadCloser, err error) {
	var absPath, appDir string

	absPath, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	zipper := appfiles.ApplicationZipper{}
	if zipper.IsZipFile(absPath) {
		appDir, err = ioutil.TempDir("", "forge-app-zip")
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				os.RemoveAll(appDir)
				return
			}
			app = &closeWrapper{
				ReadCloser: app,
				After: func() error {
					return os.RemoveAll(appDir)
				},
			}
		}()
		if err := zipper.Unzip(absPath, appDir); err != nil {
			return nil, err
		}
	} else {
		appDir, err = filepath.EvalSymlinks(absPath)
		if err != nil {
			return nil, err
		}
	}
	files, err := appFiles(appDir, excludes)
	if err != nil {
		return nil, err
	}
	return archive.Tar(appDir, files)
}

type closeWrapper struct {
	io.ReadCloser
	After func() error
}

func (c *closeWrapper) Close() (err error) {
	defer func() {
		if afterErr := c.After(); err == nil {
			err = afterErr
		}
	}()
	return c.ReadCloser.Close()
}

func appFiles(path string, excludes []string) ([]string, error) {
	var files []string
	err := appfiles.ApplicationFiles{}.WalkAppFiles(path, func(relpath, _ string) error {
		for _, excludePattern := range excludes {
			if regexp.MustCompile(excludePattern).MatchString(relpath) {
				return nil
			}
		}
		files = append(files, relpath)
		return nil
	})
	return files, err
}
