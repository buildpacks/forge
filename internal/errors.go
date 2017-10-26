package internal

import "errors"

var (
	ErrNetwork     = errors.New("no network connection")
	ErrUnavailable = errors.New("unavailable")
)
