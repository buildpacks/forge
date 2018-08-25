package docker

import (
	"encoding/json"
	"io"

	eng "github.com/buildpack/forge/engine"
	"github.com/buildpack/forge/engine/docker/httpsocket"
)

type image struct {
	exit   <-chan struct{}
	docker *httpsocket.Client
}

func (e *engine) NewImage() eng.Image {
	return &image{e.exit, e.docker}
}

func (i *image) Build(tag string, dockerfile eng.Stream) <-chan eng.Progress {
	// defer dockerfile.Close()
	// ctx := context.Background()
	progress := make(chan eng.Progress, 1)

	// dockerfileTar, err := tarFile("Dockerfile", dockerfile, dockerfile.Size, 0644)
	// if err != nil {
	// 	progress <- progressError{err}
	// 	close(progress)
	// 	return progress
	// }
	// response, err := i.docker.ImageBuild(ctx, dockerfileTar, types.ImageBuildOptions{
	// 	Tags:        []string{tag},
	// 	PullParent:  true,
	// 	Remove:      true,
	// 	ForceRemove: true,
	// })
	// if err != nil {
	// 	progress <- progressError{err}
	// 	close(progress)
	// 	return progress
	// }
	// go i.checkBody(response.Body, progress)
	return progress
}

func (i *image) Pull(ref string) <-chan eng.Progress {
	// ctx := context.Background()
	progress := make(chan eng.Progress, 1)

	// body, err := i.docker.ImagePull(ctx, ref, types.ImagePullOptions{})
	// if err != nil {
	// 	progress <- progressError{err}
	// 	close(progress)
	// 	return progress
	// }
	// go i.checkBody(body, progress)
	return progress
}

func (i *image) Push(ref string, creds eng.RegistryCreds) <-chan eng.Progress {
	// ctx := context.Background()
	progress := make(chan eng.Progress, 1)

	// credsJSON, err := json.Marshal(creds)
	// if err != nil {
	// 	progress <- progressError{err}
	// 	close(progress)
	// 	return progress
	// }
	// body, err := i.docker.ImagePush(ctx, ref, types.ImagePushOptions{
	// 	RegistryAuth: base64.StdEncoding.EncodeToString(credsJSON),
	// })
	// if err != nil {
	// 	progress <- progressError{err}
	// 	close(progress)
	// 	return progress
	// }
	// go i.checkBody(body, progress)
	return progress
}

func (i *image) Delete(id string) error {
	// ctx := context.Background()
	// _, err := i.docker.ImageRemove(ctx, id, types.ImageRemoveOptions{
	// 	Force:         true,
	// 	PruneChildren: true,
	// })
	// return err
	return nil
}

func (i *image) checkBody(body io.ReadCloser, progress chan<- eng.Progress) {
	defer body.Close()
	defer close(progress)

	decoder := json.NewDecoder(body)
	for {
		select {
		case <-i.exit:
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
