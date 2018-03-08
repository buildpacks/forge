package forge

import (
	"github.com/sclevine/forge/engine"
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
	AppConfig *AppConfig
}

// TODO: use build instead of commit
func (e *Exporter) Export(config *ExportConfig) (imageID string, err error) {
	if err := e.pull(config.Stack); err != nil {
		return "", err
	}

	containerConfig, err := e.buildConfig(config.AppConfig, config.Stack)
	if err != nil {
		return "", err
	}
	contr, err := e.engine.NewContainer(containerConfig)
	if err != nil {
		return "", err
	}
	defer contr.Close()

	if err := contr.StreamTarTo(config.Droplet, "/home/vcap"); err != nil {
		return "", err
	}
	return contr.Commit(config.Ref)
}

func (e *Exporter) pull(stack string) error {
	return e.Loader.Loading("Image", e.engine.NewImage().Pull(stack))
}

func (e *Exporter) buildConfig(app *AppConfig, stack string) (*engine.ContainerConfig, error) {
	env := map[string]string{}
	if app.Name != "" {
		env["PACK_APP_NAME"] = app.Name
	}

	return &engine.ContainerConfig{
		Name:       app.Name,
		Hostname:   app.Name,
		Env:        mapToEnv(mergeMaps(env, app.RunningEnv, app.Env)),
		Image:      stack,
		WorkingDir: "/home/vcap/app",
		Entrypoint: []string{"/packs/launcher", app.Command},
	}, nil
}
