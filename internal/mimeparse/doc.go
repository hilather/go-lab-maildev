// Package mimeparse adapts github.com/emersion/go-message into model types.
//
// This is the only production package that may import go-message. Parse never
// fails closed: malformed MIME is returned with Raw intact and ParseWarning set.
package mimeparse
