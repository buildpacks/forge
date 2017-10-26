package forge

import (
	"github.com/sclevine/forge/engine"
)

type noopLoader struct{}

func (noopLoader) Loading(_ string, progress <-chan engine.Progress) error {
	for range progress {
	}
	return nil
}
