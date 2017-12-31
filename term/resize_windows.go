package term

import (
	"os"
	"syscall"
	"time"
)

func resized() (c <-chan os.Signal, done func()) {
	r := make(chan os.Signal, 16)
	ticker := time.NewTicker(250 * time.Millisecond)

	go func() {
		defer close(r)
		for range ticker.C {
			r <- syscall.Signal(-1)
		}
	}()

	return r, ticker.Stop
}
