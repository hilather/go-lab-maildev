package store

import "errors"

var (
	// ErrFull is mapped by SMTP to 452 4.3.1.
	ErrFull = errors.New("store full")
	// ErrStaleEpoch is mapped by SMTP to 451 4.3.2.
	ErrStaleEpoch = errors.New("stale store epoch")
	// ErrTooLarge is mapped by SMTP to 552 5.3.4 (single message over maxBytes).
	ErrTooLarge = errors.New("message exceeds store maxBytes")
	// ErrNotFound is a missing inbox id.
	ErrNotFound = errors.New("message not found")
)
