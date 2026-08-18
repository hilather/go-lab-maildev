// Package perf is the GA-001 soak harness: accept N messages, Wait, Wipe.
//
// Default N is CI-safe. Operators raise it with -soak-n or LABMAIL_SOAK_N.
// This is a lab sink soak, not a public-MTA load test.
package perf
