package forge

import (
	"github.com/sclevine/forge/engine"
	"fmt"
)

type Exporter struct {
	Loader Loader
	engine Engine
}

func NewExporter(engine Engine) *Exporter {
	return &Exporter{
		Loader: noopLoader{},
		engine: engine,
	}
}

type ExportConfig struct {
	Droplet   engine.Stream
	Stack     string
	Ref       string
	RootPath  string
	HomePath  string
	AppConfig *AppConfig
}

// TODO: use build instead of commit
func (e *Exporter) Export(config *ExportConfig) (imageID string, err error) {
	rootPath := config.RootPath
	if rootPath == "" {
		rootPath = "/home/vcap"
	}

	homePath := config.HomePath
	if homePath == "" {
		homePath = "app"
	}

	// We don't want to use `filepath.Join` because it will be platform specific and we need this work on Linux
	// But `filepath.Join` will give us something like `\app`.
	workingDir := fmt.Sprintf("/%s/%s", rootPath, homePath)

	containerConfig, err := e.buildConfig(config.AppConfig, workingDir, config.Stack)
	if err != nil {
		return "", err
	}
	contr, err := e.engine.NewContainer(containerConfig)
	if err != nil {
		return "", err
	}
	defer contr.Close()

	if err := contr.StreamTarTo(config.Droplet, rootPath); err != nil {
		return "", err
	}
	return contr.Commit(config.Ref)
}

func (e *Exporter) buildConfig(app *AppConfig, workingDir, stack string) (*engine.ContainerConfig, error) {
	env := map[string]string{}
	if app.Name != "" {
		env["PACK_APP_NAME"] = app.Name
	}

	return &engine.ContainerConfig{
		Name:       app.Name,
		Hostname:   app.Name,
		Env:        mapToEnv(mergeMaps(env, app.RunningEnv, app.Env)),
		Image:      stack,
		WorkingDir: workingDir,
		Entrypoint: []string{"/packs/launcher", app.Command},
		SkipProxy:  true,
	}, nil
}
