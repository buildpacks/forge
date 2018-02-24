package forge

import "os"

type (
	MockEngine forgeEngine
	MockImage  forgeImage
)

func NewTestRunner(engine MockEngine, image MockImage) *Runner {
	return &Runner{
		Logs:   os.Stdout,
		Loader: noopLoader{},
		engine: engine,
		image:  image,
	}
}

func NewTestStager(engine MockEngine, image MockImage) *Stager {
	return &Stager{
		Logs:   os.Stdout,
		Loader: noopLoader{},
		engine: engine,
		image:  image,
	}
}

func NewTestForwarder(engine MockEngine) *Forwarder {
	return &Forwarder{
		Logs:   os.Stdout,
		engine: engine,
	}
}
