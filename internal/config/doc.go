// Package config decodes, normalizes, and schema-validates LabMail YAML and JSON.
//
// Decode rejects unknown fields at every nesting level and reserved relay
// keys after dash/underscore/case normalize. Normalize materializes defaults
// (copy-on-write). Validate returns domainerr.validation_failed with
// fieldViolations. Canonical export hashes materialized JSON; comments are
// not preserved.
package config
