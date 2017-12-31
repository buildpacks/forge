package forge

import (
	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"

	"github.com/sclevine/forge/engine"
)

const dockerfile = `
FROM {{.Stack}}

RUN \
  apt-get update && \
  apt-get -y install sshpass && \
  apt-get clean

RUN \
  curl -L "https://storage.googleapis.com/golang/go1.9.1.linux-amd64.tar.gz" | tar -C /usr/local -xz && \
  git -C /tmp clone --single-branch https://github.com/cloudfoundry/diego-release && \
  cd /tmp/diego-release && \
  git checkout "v1.32.0" && \
  git submodule update --init --recursive \
    src/code.cloudfoundry.org/archiver \
    src/code.cloudfoundry.org/buildpackapplifecycle \
    src/code.cloudfoundry.org/bytefmt \
    src/code.cloudfoundry.org/cacheddownloader \
    src/code.cloudfoundry.org/goshims \
    src/code.cloudfoundry.org/lager \
    src/code.cloudfoundry.org/systemcerts \
    src/github.com/cloudfoundry-incubator/credhub-cli \
    src/gopkg.in/yaml.v2 && \
  export PATH=/usr/local/go/bin:$PATH && \
  export GOPATH=/tmp/diego-release && \
  CGO_ENABLED=0 go build -a -installsuffix static -o /tmp/lifecycle/builder code.cloudfoundry.org/buildpackapplifecycle/builder && \
  CGO_ENABLED=0 go build -a -installsuffix static -o /tmp/lifecycle/launcher code.cloudfoundry.org/buildpackapplifecycle/launcher && \
  CGO_ENABLED=0 go build -a -installsuffix static -o /tmp/lifecycle/shell code.cloudfoundry.org/buildpackapplifecycle/shell/shell && \
  rm -rf /tmp/diego-release /usr/local/go

USER vcap

RUN mkdir -p /tmp/app /tmp/cache /home/vcap/tmp /tmp/buildpacks

{{range .Buildpacks}}
RUN curl -L -o /tmp/buildpack.zip "{{.URL}}" && unzip /tmp/buildpack.zip -d /tmp/buildpacks/{{.MD5}} && rm /tmp/buildpack.zip
{{end}}
`

type dockerEngine struct {
	Docker *docker.Client
	Exit   <-chan struct{}
}

func (d *dockerEngine) NewContainer(name string, config *container.Config, hostConfig *container.HostConfig) (Container, error) {
	contr, err := engine.NewContainer(d.Docker, name, config, hostConfig)
	if err != nil {
		return nil, err
	}
	contr.Exit = d.Exit
	return contr, nil
}
