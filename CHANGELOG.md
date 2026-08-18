# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- In-tree plain SMTP sink (`internal/smtp/{codec,server}`): greeting, HELO/EHLO, MAIL/RCPT/DATA/RSET/NOOP/QUIT/HELP, VRFY=252, EXPN=502, advertised SIZE/8BITMIME/SMTPUTF8/ENHANCEDSTATUSCODES, session and in-flight DATA caps. Accepted mail is discarded to `store.Null`. `labmail serve --config` binds SMTP. Interop: `net/smtp.SendMail` against localhost. No AUTH, STARTTLS, or implicit TLS.
- Fail-closed `labmail.dev/v1alpha1` YAML compiler: `KnownFields(true)`, reserved relay-key reject, default materialization, canonical revision hash, JSON Schema at `api/jsonschema/labmail.dev.v1alpha1.json`, and `labmail validate` / `canonicalize`.
- Repository foundation: Apache-2.0 license, Go 1.26 module `github.com/hilather/go-lab-maildev`, stub `labmail` CLI (`version` / `help` only), Makefile, fail-closed CI (format, lint, unit, docs), and the LabMail 1.0 design pack (`docs/01`–`13` + ADRs 0001–0007).
- No queryable inbox, REST, MCP, auth, UI, or container image yet.

### Changed

- None.

### Fixed

- `labmail serve` and `smtp/server.New` fail closed when compiled YAML asks for SMTP AUTH or TLS (those land in SMTP-001b).

### Removed or deprecated

- None.
