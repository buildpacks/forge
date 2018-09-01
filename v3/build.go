package v3

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	eng "github.com/buildpack/forge/engine"
)

type Builder struct {
	engine          eng.Engine
	detectImage     string
	launchVolume    string
	workspaceVolume string
	cacheVolume     string
	tmpDirBase      string
}

func NewBuilder(engine eng.Engine, detectImage, uuid, appUUID string) (*Builder, error) {
	tmpDir, err := ioutil.TempDir("", "pack.build.")
	if err != nil {
		return nil, err
	}
	return &Builder{
		engine:          engine,
		detectImage:     detectImage,
		launchVolume:    fmt.Sprintf("pack-launch-%s", uuid),
		workspaceVolume: fmt.Sprintf("pack-workspace-%s", uuid),
		cacheVolume:     fmt.Sprintf("pack-cache-%s", appUUID),
		tmpDirBase:      tmpDir,
	}, nil
}

func (b *Builder) Close() {
	b.engine.RemoveVolume(b.launchVolume)
	b.engine.RemoveVolume(b.workspaceVolume)
	if b.tmpDirBase != "" {
		os.RemoveAll(b.tmpDirBase)
	}
}

func (b *Builder) UploadToLaunch(tr io.Reader) error {
	cont, err := b.engine.NewContainer(&eng.ContainerConfig{
		Name:  "pack-upload",
		Image: b.detectImage,
		Binds: []string{
			b.launchVolume + ":/launch",
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

func (b *Builder) ExportVolume(path string) (io.ReadCloser, error) {
	cont, err := b.engine.NewContainer(&eng.ContainerConfig{
		Name:  "pack-export",
		Image: b.detectImage,
		Binds: []string{
			b.launchVolume + ":/launch",
		},
	})
	if err != nil {
		return nil, err
	}
	defer cont.Close()

	return cont.StreamTarFrom(path)
}

// func (b *Builder) Detect() (lifecycle.BuildpackGroup, error) {
// 	detectCont, err := b.Engine.NewContainer(&eng.ContainerConfig{
// 		Name:  "pack-detect",
// 		Image: b.DetectImage,
// 		Binds: []string{
// 			b.LaunchVolume + ":/launch",
// 			b.WorkspaceVolume + ":/workspace",
// 		},
// 	})
// 	if err != nil {
// 		return lifecycle.BuildpackGroup{}, err
// 	}
// 	defer detectCont.Close()
//
// 	if exitStatus, err := detectCont.Start("", os.Stdout, nil); err != nil {
// 		return lifecycle.BuildpackGroup{}, err
// 	} else if exitStatus != 0 {
// 		return lifecycle.BuildpackGroup{}, fmt.Errorf("detect failed with: %d", exitStatus)
// 	}
//
// 	return b.GroupToml(detectCont)
// }
//
// func (b *Builder) Analyze(group lifecycle.BuildpackGroup) error {
// 	tmpDir, err := b.tmpDir("Analyze")
// 	if err != nil {
// 		return err
// 	}
//
// 	origImage, err := readImage(b.RepoName, !b.Publish)
// 	if err != nil {
// 		return err
// 	}
//
// 	if origImage == nil {
// 		// no previous image to analyze
// 		return nil
// 	}
//
// 	analyzer := &lifecycle.Analyzer{
// 		Buildpacks: group.Buildpacks,
// 		Out:        os.Stdout,
// 		Err:        os.Stderr,
// 	}
// 	if err := analyzer.Analyze(tmpDir, origImage); err != nil {
// 		return packs.FailErrCode(err, packs.CodeFailedBuild)
// 	}
//
// 	return b.UploadDirToVolume(tmpDir, "/launch")
// }
//
// func (b *Builder) Build(group lifecycle.BuildpackGroup) error {
// 	buildCont, err := b.Engine.NewContainer(&eng.ContainerConfig{
// 		Name:  "pack-build",
// 		Image: group.BuildImage,
// 		Binds: []string{
// 			b.LaunchVolume + ":/launch",
// 			b.WorkspaceVolume + ":/workspace",
// 			b.CacheVolume + ":/cache",
// 		},
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	defer buildCont.Close()
// 	if exitStatus, err := buildCont.Start("", os.Stdout, nil); err != nil {
// 		return err
// 	} else if exitStatus != 0 {
// 		return fmt.Errorf("build failed with: %d", exitStatus)
// 	}
// 	return nil
// }
//
// func (b *Builder) GroupToml(container eng.Container) (lifecycle.BuildpackGroup, error) {
// 	r, err := container.StreamFileFrom("/workspace/group.toml")
// 	if err != nil {
// 		return lifecycle.BuildpackGroup{}, err
// 	}
//
// 	txt, err := ioutil.ReadAll(r)
// 	if err != nil {
// 		return lifecycle.BuildpackGroup{}, err
// 	}
//
// 	var group lifecycle.BuildpackGroup
// 	if _, err := toml.Decode(string(txt), &group); err != nil {
// 		return lifecycle.BuildpackGroup{}, err
// 	}
//
// 	return group, nil
// }
//
// func (b *Builder) Export(group lifecycle.BuildpackGroup) (string, error) {
// 	tmpDir, err := b.tmpDir("Export")
// 	if err != nil {
// 		return "", err
// 	}
//
// 	localLaunchDir, err := b.ExportVolume("/launch")
// 	if err != nil {
// 		return "", err
// 	}
//
// 	origImage, err := readImage(b.RepoName, !b.Publish)
// 	if err != nil {
// 		return "", err
// 	}
//
// 	stackImage, err := readImage(group.RunImage, !b.Publish)
// 	if err != nil || stackImage == nil {
// 		return "", packs.FailErr(err, "get image for", group.RunImage)
// 	}
//
// 	var repoStore img.Store
// 	if b.Publish {
// 		repoStore, err = img.NewRegistry(b.RepoName)
// 	} else {
// 		repoStore, err = img.NewDaemon(b.RepoName)
// 	}
// 	if err != nil {
// 		return "", packs.FailErr(err, "access", b.RepoName)
// 	}
//
// 	exporter := &lifecycle.Exporter{
// 		Buildpacks: group.Buildpacks,
// 		TmpDir:     tmpDir,
// 		Out:        os.Stdout,
// 		Err:        os.Stderr,
// 	}
// 	newImage, err := exporter.Export(
// 		localLaunchDir,
// 		stackImage,
// 		origImage,
// 	)
// 	if err != nil {
// 		return "", packs.FailErrCode(err, packs.CodeFailedBuild)
// 	}
//
// 	if err := repoStore.Write(newImage); err != nil {
// 		return "", packs.FailErrCode(err, packs.CodeFailedUpdate, "write")
// 	}
//
// 	sha, err := newImage.Digest()
// 	if err != nil {
// 		return "", packs.FailErr(err, "calculating image digest")
// 	}
//
// 	return sha.String(), nil
// }
//
// func (b *Builder) tmpDir(name string) (string, error) {
// 	if b.tmpDirBase == "" {
// 		return "", fmt.Errorf("%s expected a temp dir", name)
// 	}
// 	tmpDir := filepath.Join(b.tmpDirBase, name)
// 	return tmpDir, os.Mkdir(tmpDir, 0777)
// }
