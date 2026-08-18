// Package compiler compiles a LabMail document into a canonical snapshot.
//
// Compile calls config.Normalize + config.Validate and hashes the canonical
// spec. The returned Result is immutable; callers must not mutate Canonical.
// STA-001 will wrap this in an atomic snapshot store.
package compiler
