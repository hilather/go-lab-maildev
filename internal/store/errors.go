package store

import "errors"

var (
	// ErrFull is mapped by SMTP to 452 4.3.1.
	ErrFull = errors.New("store full")
	// ErrStaleEpoch is mapped by SMTP to 451 4.3.2.
	ErrStaleEpoch = errors.New("stale store epoch")
)
