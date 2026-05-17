package pool

import "errors"

var (
	// ErrNotStarted is returned when Submit is called before Start.
	ErrNotStarted = errors.New("pool not started")
	// ErrClosed is returned when Submit is called after Shutdown begins.
	ErrClosed = errors.New("pool is closed")
)
