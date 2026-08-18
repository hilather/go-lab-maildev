// Package compiler compiles a LabMail document into a canonical snapshot.
//
// Compile calls config.Normalize + config.Validate and hashes the canonical
// spec. The returned Snapshot is immutable; callers must not mutate Canonical.
// internal/snapshot.Store holds the live pointer that SMTP re-reads.
package compiler
