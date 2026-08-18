// Package snapshot holds the compiled immutable config snapshot and its atomic store.
//
// SMTP MAIL/RCPT/DATA load the active snapshot once per command. The inbox is
// not part of the snapshot. Callers must not mutate Canonical after Compile.
package snapshot
