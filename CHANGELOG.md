# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- In-tree SMTP sink (`internal/smtp/{codec,server}`): greeting, HELO/EHLO, MAIL/RCPT/DATA/RSET/NOOP/QUIT/HELP, VRFY=252, EXPN=502, advertised SIZE/8BITMIME/SMTPUTF8/ENHANCEDSTATUSCODES, optional AUTH PLAIN/LOGIN (`smtp.auth.mode=plain_login`) and STARTTLS (`smtp.tls.mode=starttls`, optional or required). When STARTTLS is required, AUTH is withheld and rejected on cleartext so the lab password is never accepted before the handshake. Session and in-flight DATA caps. Accepted mail is discarded to `store.Null`. `labmail serve --config` binds SMTP. Interop: `net/smtp.SendMail` against localhost (default YAML still has no AUTH/TLS). Implicit SMTPS remains rejected.
- Fail-closed `labmail.dev/v1alpha1` YAML compiler: `KnownFields(true)`, reserved relay-key reject, default materialization, canonical revision hash, JSON Schema at `api/jsonschema/labmail.dev.v1alpha1.json`, and `labmail validate` / `canonicalize`.
- Repository foundation: Apache-2.0 license, Go 1.26 module `github.com/hilather/go-lab-maildev`, stub `labmail` CLI (`version` / `help` only), Makefile, fail-closed CI (format, lint, unit, docs), and the LabMail 1.0 design pack (`docs/01`–`13` + ADRs 0001–0007).
- No queryable inbox, REST, MCP, auth, UI, or container image yet.

### Changed

- None.

### Fixed

- None.

### Removed or deprecated

- None.
