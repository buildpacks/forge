module github.com/buildpack/forge

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20180108190415-b31f603f5e1e // indirect
	code.cloudfoundry.org/cli v6.38.0+incompatible
	code.cloudfoundry.org/gofileutils v0.0.0-20170111115228-4d0c80011a0f // indirect
	github.com/Microsoft/go-winio v0.4.10 // indirect
	github.com/SermoDigital/jose v0.9.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/containerd/continuity v0.0.0-20180814194400-c7c5070e6f6e // indirect
	github.com/cyphar/filepath-securejoin v0.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.6.2+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.3.0
	github.com/docker/go-units v0.3.3 // indirect
	github.com/fatih/color v1.7.0 // indirect
	github.com/gogo/protobuf v1.1.1 // indirect
	github.com/golang/mock v0.0.0-20180211072722-58cd061d0938
	github.com/lunixbochs/vtclean v0.0.0-20180621232353-2d01aacdc34a // indirect
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.3 // indirect
	github.com/mattn/go-runewidth v0.0.3 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v0.0.0-20180216170043-9008c7b79f96
	github.com/onsi/gomega v0.0.0-20180216134830-fcebc62b30a9
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/pkg/errors v0.8.0 // indirect
	github.com/stretchr/testify v1.2.2 // indirect
	github.com/vito/go-interact v0.0.0-20171111012221-fa338ed9e9ec // indirect
	golang.org/x/crypto v0.0.0-20180820150726-614d502a4dac // indirect
	golang.org/x/net v0.0.0-20180821023952-922f4815f713 // indirect
	golang.org/x/sys v0.0.0-20180223165747-88d2dcc51026 // indirect
	golang.org/x/text v0.3.0 // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.25 // indirect
	gopkg.in/yaml.v2 v2.1.1
)

replace (
	github.com/docker/distribution v2.6.2+incompatible => ./engine/docker/vendor/github.com/docker/distribution/
	github.com/docker/docker v1.13.1 => ./engine/docker/vendor/github.com/docker/docker/
)
