// Package server is the plain SMTP receive listener and session state machine.
//
// AUTH PLAIN/LOGIN and STARTTLS are optional. Implicit SMTPS is rejected.
// Messages are handed to a store.Sink (typically store.Null).
package server
