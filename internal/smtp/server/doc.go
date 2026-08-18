// Package server is the plain SMTP receive listener and session state machine.
//
// AUTH, STARTTLS, and implicit TLS are not implemented in this slice.
// Messages are handed to a store.Sink (typically store.Null).
package server
