// Package store is the receive-only inbox surface used by SMTP.
//
// Sink is the insert/epoch API. Memory is the bounded ULID inbox. Null
// acknowledges mail and retains nothing. ReplaceCaps implements
// replaceStoreCaps shrink rules; Wipe is the only epoch bump.
package store
