package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
)

type Image struct {
	Docker *docker.Client
	Exit   <-chan struct{}
}

type RegistryCreds struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Email         string `json:"email"`
	ServerAddress string `json:"serveraddress"`
}

type Progress interface {
	Status() (string, error)
}

func (i *Image) Build(tag string, dockerfile Stream) <-chan Progress {
	defer dockerfile.Close()
	ctx := context.Background()
	progress := make(chan Progress, 1)

	dockerfileTar, err := tarFile("Dockerfile", dockerfile, dockerfile.Size, 0644)
	if err != nil {
		progress <- progressError{err}
		close(progress)
		return progress
	}
	response, err := i.Docker.ImageBuild(ctx, dockerfileTar, types.ImageBuildOptions{
		Tags:        []string{tag},
		PullParent:  true,
		Remove:      true,
		ForceRemove: true,
	})
	if err != nil {
		progress <- progressError{err}
		close(progress)
		return progress
	}
	go i.checkBody(response.Body, progress)
	return progress
}

func (i *Image) Pull(ref string) <-chan Progress {
	ctx := context.Background()
	progress := make(chan Progress, 1)

	body, err := i.Docker.ImagePull(ctx, ref, types.ImagePullOptions{})
	if err != nil {
		progress <- progressError{err}
		close(progress)
		return progress
	}
	go i.checkBody(body, progress)
	return progress
}

func (i *Image) Push(ref string, creds RegistryCreds) <-chan Progress {
	ctx := context.Background()
	progress := make(chan Progress, 1)

	credsJSON, err := json.Marshal(creds)
	if err != nil {
		progress <- progressError{err}
		close(progress)
		return progress
	}
	body, err := i.Docker.ImagePush(ctx, ref, types.ImagePushOptions{
		RegistryAuth: base64.StdEncoding.EncodeToString(credsJSON),
	})
	if err != nil {
		progress <- progressError{err}
		close(progress)
		return progress
	}
	go i.checkBody(body, progress)
	return progress
}

func (i *Image) Delete(id string) error {
	ctx := context.Background()
	_, err := i.Docker.ImageRemove(ctx, id, types.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	})
	return err
}

func (i *Image) checkBody(body io.ReadCloser, progress chan<- Progress) {
	defer body.Close()
	defer close(progress)

	decoder := json.NewDecoder(body)
	for {
		select {
		case <-i.Exit:
			progress <- progressErrorString("interrupted")
			return
		default:
			var stream struct {
				Error    string
				Progress string
			}
			if err := decoder.Decode(&stream); err != nil {
				if err != io.EOF {
					progress <- progressError{err}
				}
				return
			}
			if stream.Error != "" {
				progress <- progressErrorString(stream.Error)
				return
			}
			if stream.Progress == "" {
				progress <- progressNA{}
			} else {
				progress <- progressMsg(stream.Progress)
			}
		}
	}
}
