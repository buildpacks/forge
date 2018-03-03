package docker

import "errors"

type progressMsg string

func (p progressMsg) Status() (string, error) {
	return string(p), nil
}

type progressNA struct{}

func (p progressNA) Status() (string, error) {
	return "N/A", nil
}

type progressError struct{ error }

func (p progressError) Status() (string, error) {
	return "", p.error
}

type progressErrorString string

func (p progressErrorString) Status() (string, error) {
	return "", errors.New(string(p))
}
