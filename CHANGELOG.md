# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- Fail-closed `labmail.dev/v1alpha1` YAML compiler: `KnownFields(true)`, reserved relay-key reject, default materialization, canonical revision hash, JSON Schema at `api/jsonschema/labmail.dev.v1alpha1.json`, and `labmail validate` / `canonicalize`.
- Repository foundation: Apache-2.0 license, Go 1.26 module `github.com/hilather/go-lab-maildev`, stub `labmail` CLI (`version` / `help` only), Makefile, fail-closed CI (format, lint, unit, docs), and the LabMail 1.0 design pack (`docs/01`–`13` + ADRs 0001–0007).
- No SMTP listener, message store, REST, MCP, auth, UI, or container image yet.

### Changed

- None.

### Fixed

- None.

### Removed or deprecated

- None.
