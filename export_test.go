package forge

import "os"

type (
	MockEngine    forgeEngine
	MockImage     forgeImage
	MockVersioner versioner
)

func NewTestRunner(engine MockEngine, image MockImage) *Runner {
	return &Runner{
		Logs:   os.Stdout,
		Loader: noopLoader{},
		engine: engine,
		image:  image,
	}
}

func NewTestStager(versioner MockVersioner, engine MockEngine, image MockImage) *Stager {
	return &Stager{
		ImageTag:  "forge",
		Logs:      os.Stdout,
		Loader:    noopLoader{},
		versioner: versioner,
		engine:    engine,
		image:     image,
	}
}

func NewTestForwarder(engine MockEngine) *Forwarder {
	return &Forwarder{
		Logs:   os.Stdout,
		engine: engine,
	}
}
