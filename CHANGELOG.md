# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- maildev 2.2.1 compat adapter (`internal/control/compat`) on the same management listener as `/v1` when `spec.listeners.management.compatEnabled` is true (default): `GET /email` JSON **array** (bodies omitted; `?skip=` and dotted filters), `GET /email/:id` marks read, `DELETE /email/:id` and `DELETE /email/all`, `GET /email/:id/html` (preview CSP), `GET /email/:id/attachment/:filename`, `GET /healthz`, redacted `GET /config` (`receiveOnly: true`). `POST /email/:id/relay` is always 403 `receive_only`. Auth is stubbed via a fake principal injector (401 + Basic is SEC-001). Documented deltas: ULID ids, sha256 attachment checksum, no `stream`.
- Native REST `/v1` (`internal/control/rest`) over `app.Service`: problem+json domain codes (`cursor_stale`, `store_over_new_cap` are first-class, not wrapped as `validation_failed`), HMAC list cursors, messages wait/extract (frozen RE2), preview CSP + `cid:` → `data:` rewrite, plan/apply JSON, health/ready, capability catalog, generated OpenAPI (`api/openapi/v1.json`) and capability manifest (`api/capabilities/v1.json`). `labmail serve` binds management HTTP from YAML (`--management-listen ADDR|off`). Auth is stubbed open. Session and MCP are later PRs. Native `GET /v1/messages/{id}` defaults `markRead=false`.
- HTTP-less `internal/app.Service` with atomic config snapshot, plan/apply (coarse ops), reset that rereads YAML and wipes the inbox (Wipe is the only epoch bump; in-flight Insert → 451), `replaceStoreCaps` shrink rules, and an in-process audit ring on reset/delete/apply. SMTP insert stays on the data plane. `labmail serve` boots through `app.Service`; MAIL/RCPT/DATA re-read the live snapshot.
- Bounded memory inbox (`internal/store.Memory`): Crockford ULID ids, MIME extract via `internal/mimeparse` (the only `go-message` importer), stacked resident caps (raw + decoded), `fullPolicy: reject` → SMTP 452, single-message over `maxBytes` → 552, Wait + timeout, Wipe epoch (stale Insert → 451), optional tmpfs spill. Malformed MIME is still stored. `labmail serve` inserts into Memory, not Null.
- In-tree SMTP sink (`internal/smtp/{codec,server}`): greeting, HELO/EHLO, MAIL/RCPT/DATA/RSET/NOOP/QUIT/HELP, VRFY=252, EXPN=502, advertised SIZE/8BITMIME/SMTPUTF8/ENHANCEDSTATUSCODES, optional AUTH PLAIN/LOGIN (`smtp.auth.mode=plain_login`) and STARTTLS (`smtp.tls.mode=starttls`, optional or required). When STARTTLS is required, AUTH is withheld and rejected on cleartext so the lab password is never accepted before the handshake. Session and in-flight DATA caps. `labmail serve --config` binds SMTP. Interop: `net/smtp.SendMail` against localhost (default YAML still has no AUTH/TLS). Implicit SMTPS remains rejected.
- Fail-closed `labmail.dev/v1alpha1` YAML compiler: `KnownFields(true)`, reserved relay-key reject, default materialization, canonical revision hash, JSON Schema at `api/jsonschema/labmail.dev.v1alpha1.json`, and `labmail validate` / `canonicalize`.
- Repository foundation: Apache-2.0 license, Go 1.26 module `github.com/hilather/go-lab-maildev`, stub `labmail` CLI (`version` / `help` only), Makefile, fail-closed CI (format, lint, unit, docs), and the LabMail 1.0 design pack (`docs/01`–`13` + ADRs 0001–0007).
- MCP, auth, UI, and the container image are not implemented yet. Inbox list/get/delete/wait/extract exist on `app.Service`, native `/v1`, and maildev `/email` compat.

### Changed

- None.

### Fixed

- Reset preflights store options (including a creatable spill directory) and installs new caps under one lock, so a failed reset cannot empty the inbox under the old snapshot.

### Removed or deprecated

- None.
