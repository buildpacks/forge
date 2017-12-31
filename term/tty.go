package term

import (
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/term"
)

type TTY struct {
	In  io.Reader
	Out io.Writer
}

func (t *TTY) Run(remoteIn io.Reader, remoteOut io.Writer, resize func(h, w uint16) error) error {
	inFd, inIsTerm := term.GetFdInfo(t.In)
	outFd, outIsTerm := term.GetFdInfo(t.Out)

	if inIsTerm {
		size := t.winsize(outFd)
		if err := resize(size.Height, size.Width); err != nil {
			return err
		}

		var state *term.State
		state, err := term.SetRawTerminal(outFd)
		if err == nil {
			defer term.RestoreTerminal(inFd, state)
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go copy(wg, t.Out, remoteIn)
	go copy(wg, remoteOut, t.In)

	if outIsTerm {
		resized := make(chan os.Signal, 16)

		if runtime.GOOS == "windows" {
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()

			go func() {
				defer close(resized)
				for range ticker.C {
					resized <- syscall.Signal(-1)
				}
			}()
		} else {
			defer close(resized)
			signal.Notify(resized, syscall.SIGWINCH)
			defer signal.Stop(resized)
		}

		go t.resize(resized, resize, outFd)
	}

	wg.Wait()
	return nil
}

func (t *TTY) resize(resized <-chan os.Signal, resize func(h, w uint16) error, fd uintptr) {
	var h, w uint16
	for range resized {
		size := t.winsize(fd)
		if size.Height == h && size.Width == w {
			continue
		}

		resize(size.Height, size.Width)
		h, w = size.Height, size.Width
	}
}

func (t *TTY) winsize(fd uintptr) *term.Winsize {
	size, err := term.GetWinsize(fd)
	if err != nil {
		size = &term.Winsize{
			Height: 43,
			Width:  80,
		}
	}
	return size
}

// TODO: collect copy errors
func copy(wg *sync.WaitGroup, dst io.Writer, src io.Reader) {
	io.Copy(dst, src)
	wg.Done()
}
