package forge

import (
	"github.com/buildpack/forge/engine"
)

type Exporter struct {
	engine Engine
}

func NewExporter(engine Engine) *Exporter {
	return &Exporter{engine: engine}
}

type ExportConfig struct {
	Droplet    engine.Stream
	Stack      string
	Ref        string
	OutputDir  string
	WorkingDir string
	AppConfig  *AppConfig
}

// TODO: use build instead of commit
func (e *Exporter) Export(config *ExportConfig) (imageID string, err error) {
	containerConfig, err := e.buildConfig(config.AppConfig, config.WorkingDir, config.Stack)
	if err != nil {
		return "", err
	}
	contr, err := e.engine.NewContainer(containerConfig)
	if err != nil {
		return "", err
	}
	defer contr.Close()

	if err := contr.StreamTarTo(config.Droplet, config.OutputDir); err != nil {
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
		Entrypoint: []string{"/packs/launcher"},
		Cmd:        []string{app.Command},
		SkipProxy:  true,
	}, nil
}
