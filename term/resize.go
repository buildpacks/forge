// +build !windows

package term

import (
	"os"
	"os/signal"
	"syscall"
)

func resized() (c <-chan os.Signal, done func()) {
	r := make(chan os.Signal, 16)
	signal.Notify(r, syscall.SIGWINCH)

	return r, func() {
		defer close(r)
		defer signal.Stop(r)
	}
}
