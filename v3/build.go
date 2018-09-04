package v3

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	eng "github.com/buildpack/forge/engine"
	"github.com/buildpack/lifecycle"
	"github.com/buildpack/packs"
	"github.com/buildpack/packs/img"
)

type Builder struct {
	engine          eng.Engine
	LaunchVolume    eng.Volume
	WorkspaceVolume eng.Volume
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
		LaunchVolume:    engine.NewVolume(fmt.Sprintf("pack-launch-%s", uuid), "/launch", detectImage),
		WorkspaceVolume: engine.NewVolume(fmt.Sprintf("pack-workspace-%s", uuid), "/workspace", detectImage),
		cacheVolume:     fmt.Sprintf("pack-cache-%s:/cache", appUUID),
		tmpDirBase:      tmpDir,
	}, nil
}

func (b *Builder) Close() {
	b.LaunchVolume.Close()
	b.WorkspaceVolume.Close()
	if b.tmpDirBase != "" {
		os.RemoveAll(b.tmpDirBase)
	}
}

func (b *Builder) runContainer(cfg *eng.ContainerConfig) (eng.Container, error) {
	cont, err := b.engine.NewContainer(cfg)
	if err != nil {
		return nil, err
	}
	if exitStatus, err := cont.Start("", os.Stdout, nil); err != nil {
		cont.Close()
		return nil, err
	} else if exitStatus != 0 {
		cont.Close()
		return nil, fmt.Errorf("detect failed with: %d", exitStatus)
	}
	return cont, nil
}

func (b *Builder) Detect(detectImage string) (lifecycle.BuildpackGroup, error) {
	container, err := b.runContainer(&eng.ContainerConfig{
		Name:  "pack-detect",
		Image: detectImage,
		Binds: []string{b.LaunchVolume.String(), b.WorkspaceVolume.String()},
	})
	if err != nil {
		return lifecycle.BuildpackGroup{}, err
	}
	defer container.Close()

	r, err := container.StreamFileFrom("/workspace/group.toml")
	if err != nil {
		return lifecycle.BuildpackGroup{}, err
	}
	var group lifecycle.BuildpackGroup
	if _, err := toml.DecodeReader(r, &group); err != nil {
		return lifecycle.BuildpackGroup{}, err
	}
	return group, nil
}

func (b *Builder) Analyze(repoName string, useDaemon bool, group lifecycle.BuildpackGroup) error {
	tmpDir, err := b.tmpDir("Analyze")
	if err != nil {
		return err
	}

	origImage, err := readImage(repoName, useDaemon)
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

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()
	globs, err := filepath.Glob(filepath.Join(tmpDir, "*", "*.toml"))
	if err != nil {
		return err
	}
	for _, glob := range globs {
		txt, err := ioutil.ReadFile(glob)
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name: filepath.Join("/launch", filepath.Base(filepath.Dir(glob)), filepath.Base(glob)),
			Mode: 0666,
			Size: int64(len(txt)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(txt)); err != nil {
			return err
		}
	}

	return b.LaunchVolume.Upload(bytes.NewReader(buf.Bytes()))
}

func (b *Builder) Build(group lifecycle.BuildpackGroup) error {
	container, err := b.runContainer(&eng.ContainerConfig{
		Name:  "pack-build",
		Image: group.BuildImage,
		Binds: []string{
			b.LaunchVolume.String(),
			b.WorkspaceVolume.String(),
			b.cacheVolume,
		},
	})
	container.Close()
	return err
}

func (b *Builder) Export(group lifecycle.BuildpackGroup) (string, error) {
	tmpDir, err := b.tmpDir("Export")
	if err != nil {
		return "", err
	}

	localLaunchDir, err := b.LaunchVolume.Export("/launch")
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

func (b *Builder) tmpDir(name string) (string, error) {
	if b.tmpDirBase == "" {
		return "", fmt.Errorf("%s expected a temp dir", name)
	}
	tmpDir := filepath.Join(b.tmpDirBase, name)
	return tmpDir, os.Mkdir(tmpDir, 0777)
}
