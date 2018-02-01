package engine

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
)

type Stream struct {
	io.ReadCloser
	*StreamState
}

type StreamState struct {
	Size   int64
	closed bool
}

func NewStream(data io.ReadCloser, size int64) Stream {
	return Stream{
		data,
		&StreamState{size, false},
	}
}

func (s *Stream) Out(dst io.Writer) error {
	if s.closed {
		return errors.New("closed")
	}
	defer s.ReadCloser.Close()
	n, err := io.CopyN(dst, s, s.Size)
	s.Size -= n
	return err
}

func (s *Stream) Close() error {
	if s.closed {
		return nil
	}
	if err := s.ReadCloser.Close(); err != nil {
		return err
	}
	s.closed = true
	return nil
}

func tarFile(name string, contents io.Reader, size, mode int64) (io.Reader, error) {
	tarBuffer := &bytes.Buffer{}
	tarball := tar.NewWriter(tarBuffer)
	defer tarball.Close()
	header := &tar.Header{Name: name, Size: size, Mode: mode}
	if err := tarball.WriteHeader(header); err != nil {
		return nil, err
	}
	if _, err := io.CopyN(tarball, contents, size); err != nil {
		return nil, err
	}
	return tarBuffer, nil
}
