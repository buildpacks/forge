package term

import (
	"io"
	"os"
	"sync"

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
		size := winsize(outFd)
		if err := resize(size.Height, size.Width); err != nil {
			return err
		}

		var state *term.State
		state, err := term.SetRawTerminal(inFd)
		if err == nil {
			defer term.RestoreTerminal(inFd, state)
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go copy(wg, t.Out, remoteIn)
	go copy(wg, remoteOut, t.In)

	if outIsTerm {
		r, done := resized()
		defer done()
		go t.resize(r, resize, outFd)
	}

	wg.Wait()
	return nil
}

func (t *TTY) resize(resized <-chan os.Signal, resize func(h, w uint16) error, fd uintptr) {
	var h, w uint16
	for range resized {
		size := winsize(fd)
		if size.Height == h && size.Width == w {
			continue
		}

		resize(size.Height, size.Width)
		h, w = size.Height, size.Width
	}
}

func winsize(fd uintptr) *term.Winsize {
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
