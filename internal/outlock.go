package internal

import (
	"io"
	"io/ioutil"
	"sync"
)

type LockWriter struct {
	w io.Writer
	s sync.Mutex
}

func NewLockWriter(w io.Writer) *LockWriter {
	return &LockWriter{w: w, s: sync.Mutex{}}
}

func (w *LockWriter) Write(p []byte) (n int, err error) {
	w.s.Lock()
	defer w.s.Unlock()
	return w.w.Write(p)
}

func (w *LockWriter) Disable() {
	w.s.Lock()
	defer w.s.Unlock()
	w.w = ioutil.Discard
}
