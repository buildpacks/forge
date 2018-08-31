package pack

import (
	"archive/tar"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"
	eng "github.com/buildpack/forge/engine"
	engdocker "github.com/buildpack/forge/engine/docker"
	"github.com/buildpack/lifecycle"
	"github.com/buildpack/packs"
	"github.com/buildpack/packs/img"
	"github.com/google/uuid"
)

func Build(appDir, detectImage, repoName string, publish bool) error {
	return (&BuildFlags{
		AppDir:      appDir,
		DetectImage: detectImage,
		RepoName:    repoName,
		Publish:     publish,
	}).Run()
}

type BuildFlags struct {
	AppDir      string
	DetectImage string
	RepoName    string
	Publish     bool
	// Set by init
	LaunchVolume    string
	WorkspaceVolume string
	CacheVolume     string
	Engine          eng.Engine
	TmpDir          string
}

func (b *BuildFlags) Init() error {
	var err error
	b.AppDir, err = filepath.Abs(b.AppDir)
	if err != nil {
		return err
	}

	uid := uuid.New().String()
	b.LaunchVolume = fmt.Sprintf("pack-launch-%x", uid)
	b.WorkspaceVolume = fmt.Sprintf("pack-workspace-%x", uid)
	b.CacheVolume = fmt.Sprintf("pack-cache-%x", md5.Sum([]byte(b.AppDir)))

	b.Engine, err = engdocker.New(&eng.EngineConfig{})
	if err != nil {
		return err
	}

	b.TmpDir, err = ioutil.TempDir("", "pack.build.")
	return err
}

func (b *BuildFlags) Close() {
	exec.Command("docker", "volume", "rm", "-f", b.LaunchVolume).Run()
	exec.Command("docker", "volume", "rm", "-f", b.WorkspaceVolume).Run()
	if b.TmpDir != "" {
		os.RemoveAll(b.TmpDir)
	}
}

func (b *BuildFlags) Run() error {
	if err := b.Init(); err != nil {
		return err
	}
	defer b.Close()

	waitFor(b.Engine.NewImage().Pull(b.DetectImage))
	fmt.Println("*** COPY APP TO VOLUME:")
	if err := b.UploadDirToVolume(b.AppDir, "/launch/app"); err != nil {
		return err
	}

	fmt.Println("*** DETECTING:")
	group, err := b.Detect()
	if err != nil {
		return err
	}

	fmt.Println("*** ANALYZING: Reading information from previous image for possible re-use")
	if err := b.Analyze(group); err != nil {
		return err
	}

	fmt.Println("*** BUILDING:")
	waitFor(b.Engine.NewImage().Pull(group.BuildImage))
	if err := b.Build(group); err != nil {
		return err
	}

	if !b.Publish {
		fmt.Println("*** PULLING RUN IMAGE LOCALLY:")
		waitFor(b.Engine.NewImage().Pull(group.RunImage))
	}

	fmt.Println("*** EXPORTING:")
	imgSHA, err := b.Export(group)
	if err != nil {
		return err
	}

	if b.Publish {
		fmt.Printf("\n*** Image: %s@%s\n", b.RepoName, imgSHA)
	}

	return nil
}

func (b *BuildFlags) UploadDirToVolume(srcDir, destDir string) error {
	cont, err := b.Engine.NewContainer(&eng.ContainerConfig{
		Name:  "pack-upload",
		Image: b.DetectImage,
		Binds: []string{
			b.LaunchVolume + ":/launch",
		},
		// TODO below is very problematic
		Entrypoint: []string{},
		Cmd:        []string{"chown", "-R", "packs", "/launch"},
		User:       "root",
	})
	if err != nil {
		return err
	}
	defer cont.Close()
	tr, err := createTarReader(srcDir, destDir)
	if err != nil {
		return err
	}
	if err := cont.UploadTarTo(tr, "/"); err != nil {
		return err
	}
	if exitStatus, err := cont.Start("", os.Stdout, nil); err != nil {
		return err
	} else if exitStatus != 0 {
		return fmt.Errorf("upload failed with: %d", exitStatus)
	}
	return nil
}

func (b *BuildFlags) ExportVolume(path string) (string, error) {
	tmpDir, err := b.tmpDir("ExportVolume")
	if err != nil {
		return "", err
	}

	cont, err := b.Engine.NewContainer(&eng.ContainerConfig{
		Name:  "pack-export",
		Image: b.DetectImage,
		Binds: []string{
			b.LaunchVolume + ":/launch",
		},
	})
	if err != nil {
		return "", err
	}
	defer cont.Close()

	r, err := cont.StreamTarFrom(path)
	if err != nil {
		return "", err
	}

	if err := untarReader(r, tmpDir); err != nil {
		return "", err
	}

	return tmpDir, nil
}

func (b *BuildFlags) Detect() (lifecycle.BuildpackGroup, error) {
	detectCont, err := b.Engine.NewContainer(&eng.ContainerConfig{
		Name:  "pack-detect",
		Image: b.DetectImage,
		Binds: []string{
			b.LaunchVolume + ":/launch",
			b.WorkspaceVolume + ":/workspace",
		},
	})
	if err != nil {
		return lifecycle.BuildpackGroup{}, err
	}
	defer detectCont.Close()

	if exitStatus, err := detectCont.Start("", os.Stdout, nil); err != nil {
		return lifecycle.BuildpackGroup{}, err
	} else if exitStatus != 0 {
		return lifecycle.BuildpackGroup{}, fmt.Errorf("detect failed with: %d", exitStatus)
	}

	return b.GroupToml(detectCont)
}

func (b *BuildFlags) Analyze(group lifecycle.BuildpackGroup) error {
	tmpDir, err := b.tmpDir("Analyze")
	if err != nil {
		return err
	}

	origImage, err := readImage(b.RepoName, !b.Publish)
	if err != nil {
		return err
	}

	if origImage == nil {
		// no previous image to analyze
		return nil
	}

	analyzer := &lifecycle.Analyzer{
		Buildpacks: group.Buildpacks,
		Out:        os.Stdout,
		Err:        os.Stderr,
	}
	if err := analyzer.Analyze(tmpDir, origImage); err != nil {
		return packs.FailErrCode(err, packs.CodeFailedBuild)
	}

	return b.UploadDirToVolume(tmpDir, "/launch")
}

func (b *BuildFlags) Build(group lifecycle.BuildpackGroup) error {
	buildCont, err := b.Engine.NewContainer(&eng.ContainerConfig{
		Name:  "pack-build",
		Image: group.BuildImage,
		Binds: []string{
			b.LaunchVolume + ":/launch",
			b.WorkspaceVolume + ":/workspace",
			b.CacheVolume + ":/cache",
		},
	})
	if err != nil {
		return err
	}
	defer buildCont.Close()
	if exitStatus, err := buildCont.Start("", os.Stdout, nil); err != nil {
		return err
	} else if exitStatus != 0 {
		return fmt.Errorf("build failed with: %d", exitStatus)
	}
	return nil
}

func (b *BuildFlags) GroupToml(container eng.Container) (lifecycle.BuildpackGroup, error) {
	r, err := container.StreamFileFrom("/workspace/group.toml")
	if err != nil {
		return lifecycle.BuildpackGroup{}, err
	}

	txt, err := ioutil.ReadAll(r)
	if err != nil {
		return lifecycle.BuildpackGroup{}, err
	}

	var group lifecycle.BuildpackGroup
	if _, err := toml.Decode(string(txt), &group); err != nil {
		return lifecycle.BuildpackGroup{}, err
	}

	return group, nil
}

func (b *BuildFlags) Export(group lifecycle.BuildpackGroup) (string, error) {
	tmpDir, err := b.tmpDir("Export")
	if err != nil {
		return "", err
	}

	localLaunchDir, err := b.ExportVolume("/launch")
	if err != nil {
		return "", err
	}

	origImage, err := readImage(b.RepoName, !b.Publish)
	if err != nil {
		return "", err
	}

	stackImage, err := readImage(group.RunImage, !b.Publish)
	if err != nil || stackImage == nil {
		return "", packs.FailErr(err, "get image for", group.RunImage)
	}

	var repoStore img.Store
	if b.Publish {
		repoStore, err = img.NewRegistry(b.RepoName)
	} else {
		repoStore, err = img.NewDaemon(b.RepoName)
	}
	if err != nil {
		return "", packs.FailErr(err, "access", b.RepoName)
	}

	exporter := &lifecycle.Exporter{
		Buildpacks: group.Buildpacks,
		TmpDir:     tmpDir,
		Out:        os.Stdout,
		Err:        os.Stderr,
	}
	newImage, err := exporter.Export(
		localLaunchDir,
		stackImage,
		origImage,
	)
	if err != nil {
		return "", packs.FailErrCode(err, packs.CodeFailedBuild)
	}

	if err := repoStore.Write(newImage); err != nil {
		return "", packs.FailErrCode(err, packs.CodeFailedUpdate, "write")
	}

	sha, err := newImage.Digest()
	if err != nil {
		return "", packs.FailErr(err, "calculating image digest")
	}

	return sha.String(), nil
}

func (b *BuildFlags) tmpDir(name string) (string, error) {
	if b.TmpDir == "" {
		return "", fmt.Errorf("%s expected a temp dir", name)
	}
	tmpDir := filepath.Join(b.TmpDir, name)
	return tmpDir, os.Mkdir(tmpDir, 0777)
}

// TODO share between here and create.go (and exporter).
func createTarReader(fsDir, tarDir string) (io.ReadCloser, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	err := filepath.Walk(fsDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.Mode().IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(fsDir, file)
		if err != nil {
			return err
		}

		var header *tar.Header
		if fi.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(file)
			if err != nil {
				return err
			}
			header, err = tar.FileInfoHeader(fi, target)
			if err != nil {
				return err
			}
		} else {
			header, err = tar.FileInfoHeader(fi, fi.Name())
			if err != nil {
				return err
			}
		}
		header.Name = filepath.Join(tarDir, relPath)
		fmt.Println("    ", header.Name)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if fi.Mode().IsRegular() {
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(buf.Bytes())), nil
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

func waitFor(c <-chan eng.Progress) {
	for {
		select {
		case _, ok := <-c:
			if !ok {
				return
			}
		}
	}
}
