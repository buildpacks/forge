package app

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"code.cloudfoundry.org/cli/cf/appfiles"
	"github.com/docker/docker/pkg/archive"
)

type App struct{}

func (a *App) Tar(path string) (app io.ReadCloser, err error) {
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
	files, err := appFiles(appDir)
	if err != nil {
		return nil, err
	}
	return archive.TarWithOptions(appDir, &archive.TarOptions{
		IncludeFiles: files,
	})
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

func appFiles(path string) ([]string, error) {
	var files []string
	err := appfiles.ApplicationFiles{}.WalkAppFiles(path, func(relpath, _ string) error {
		filename := filepath.Base(relpath)
		switch {
		case
			regexp.MustCompile(`^.+\.droplet$`).MatchString(filename),
			regexp.MustCompile(`^\..+\.cache$`).MatchString(filename):
			return nil
		}
		files = append(files, relpath)
		return nil
	})
	return files, err
}

