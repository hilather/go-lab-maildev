# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- None.

### Changed

- README and START-HERE rewritten as an operator-facing front door: header art, a YAML bootstrap walkthrough, and curl/MCP examples for the state loading APIs (`GET /v1/state`, `POST /v1/state:validate`, `GET /v1/state:export`, `POST /v1/state:reset`, `POST /v1/changes:plan`, `POST /v1/changes:apply`). Architecture pack, ADRs, and the program board stay linked from the documentation map.

### Fixed

- Native `/v1/**/relay` is a path-segment guard. Message and attachment ids that only contain the substring `relay` are no longer `403 receive_only`.
- Inbox `List` returns `store.ErrSpill` when a recorded spill file cannot be read, matching `Get`/`Wait` and `docs/03-message-store.md`. Missing spill files are no longer dropped from the page while `Stats` still counts them.
- `Get`/`Wait` re-check membership after spill I/O so a concurrent delete/wipe cannot return a message that is no longer in the store.
- A DATA line over 8192 octets replies `500` and aborts the transaction without closing the SMTP session.
- Plan/apply idempotency fingerprints include `expectedRevision` and `force`, so a reused key with a different concurrency token is `idempotency_conflict`.
- `spec.listeners.management.tls.enabled` terminates TLS on the management listener (TLS 1.2+). Enabling it no longer set `Secure` cookies on a cleartext bind.
- Inbox UI ignores stale overlapping list responses and keeps a 15s list watchdog while SSE is open so dropped fan-out events cannot leave the page silent.
- `replaceStoreCaps` swaps the config snapshot before applying store caps, and rolls the snapshot back if `ReplaceCaps` fails, so a failed apply cannot leave new YAML live under the old inbox limits.
- REST SSE streams and MCP long requests (`subscriptions/listen`, `mail_messages_wait`) are cancelled when apply/reset changes the compiled auth identity. MCP inbox fan-out is restarted on the same change.
- `HealthFacts` without `SetHealth` no longer assumes SMTP and management are bound; Ready stays false until `serve` installs listener facts.
- HTML parts with `Content-Disposition: attachment` stay in `Attachments` and no longer populate `msg.HTML` / the sandboxed preview.

### Removed or deprecated

- None.

## 1.0.0-rc.2

Second candidate. Tag only on a green CI SHA via the Release `tag-gate`. Notes: [docs/releases/v1.0.0-rc.2.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.2.md). Residuals: [docs/known-limitations.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/known-limitations.md).

### Added

- Optional `spec.smtp.behavior` and live `replaceSMTPBehavior` for QA handshake scripting: greeting/command delays (max 30s), drop-on-connect, close-after-verb, and per-verb first-line reply overrides (`CODE text`). Empty is a no-op. 4xx/5xx overrides skip the success path (no MAIL/RCPT state change, no DATA store, no AUTH success, no STARTTLS handshake). Existing sessions pick up a live apply on the next command. This is deterministic scripting, not a LabDNS-style random chaos engine (D16).

### Changed

- Container contract test authenticates `GET /v1/messages` with a dedicated smoke token. Ready stays unauthenticated.
- Release `tag-gate` now requires the `container-test` CI job.

### Fixed

- golangci-lint `errcheck` on `Body.Close` in `labmail healthcheck` and several tests; staticcheck findings (`Shutdown(nil)`, nil `len` check, empty HEAD branch, deprecated `parser.ParseDir`).
- RSET 4xx/5xx behavior overrides no longer clear the transaction before the reply.

### Removed or deprecated

- None.

## 1.0.0-rc.1

Candidate identity for the first tag. Tag only on a green CI SHA via the Release `tag-gate`. Notes: [docs/releases/v1.0.0-rc.1.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.1.md). Residuals: [docs/known-limitations.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/known-limitations.md).

LabMail is a lab sink, not a public MTA. Inbox UI is included (Q2). Compose/image pin in mcp-integration-lab remains a follow-up after rc.1.

### Added

- Side-by-side maildev 2.2.1 probe (`internal/compatcheck`, `TestSideBySideMaildev221`): one `net/smtp.SendMail` + `/email` + `/healthz` client against shipped `labmail serve` and, when Docker is available, live `maildev/maildev:2.2.1`. Shared-shape fields must match; ULID ids, sha256 checksums, omitted list bodies, and relay 403 stay documented deltas.
- GA-001: committed fuzz corpora (config, SMTP codec, MIME, buildinfo), `internal/perf` soak (accept N messages, Wait, Wipe), `docs/known-limitations.md`, `docs/releases/v1.0.0-rc.1.md`, `scripts/checkchangelog`, Release workflow `tag-gate`.
- Integration-lab swap overlay (SWAP-001): full file-level BOM in `docs/13-integration-lab-swap.md`; `examples/labmail.yaml` (`allowLegacyClients: true`, Basic user frozen `admin`, no SMTP AUTH); `examples/mcpjungle/servers/labmail.json` + `groups/integration.json` (`LABMAIL_TOKEN`, append `labmail`); `examples/labinfo/services-maildev.yaml` (catalog id stays `maildev`). Bind-mounted `labmail-token` and `maildev-web-password` must be **0o644** (UID 65532). Lab smoke twin remains `TestMaildevScenarioCompat`.
- Embedded inbox SPA (`web/` + `internal/web` `go:embed`): React/TS + Vite (Node **22.14.0**), login via bearer or Basic (`POST /v1/session`), HttpOnly `labmail_session` + in-memory `X-LabMail-CSRF`, inbox list, message view (text / sandboxed HTML preview / headers / raw / attachments), status, scoped audit, gated reset. Live update is `EventSource` `GET /v1/events/stream` with a 3s `GET /v1/messages` poll fallback. Preview iframe `sandbox` has no `allow-scripts` / `allow-same-origin`. `spec.ui.enabled: false` 404s `/` and keeps REST/MCP. No Relay, send, outgoing settings, or compose. `make web-test` / `make web-build`.
- Hardened image (`Dockerfile`): `golang:1.26.6-alpine` → `scratch`, numeric `USER 65532:65532`, no shell, exec-form `HEALTHCHECK` against `GET /v1/health/ready` (not SMTP/`node`). Compose smoke [`examples/compose.smoke.yaml`](https://github.com/hilather/go-lab-maildev/blob/main/examples/compose.smoke.yaml) is read-only, `cap_drop: ALL`, `no-new-privileges`, tmpfs `/tmp`. `make test-container` / CI `container-test` assert the contract. `serve` flags: `--smtp-listen`, `--management-listen ADDR|off`, `--shutdown-timeout` (default 5s), `--pid-file`.
- Lab static bearer auth (`internal/auth`): tokens ≥256 bits compared as SHA-256 digests; default YAML `bearer_and_basic`; HTTP Basic maps onto the same `tokenRef` principal; MCP is bearer-only; unauthenticated `GET /email` is 401; `WWW-Authenticate: Bearer` (and Basic when enabled); UI session `POST/GET/DELETE /v1/session` with cookie `labmail_session` (`HttpOnly`, `SameSite=Lax`, `Secure` iff management TLS) and CSRF header `X-LabMail-CSRF` on cookie-authenticated mutations; Origin missing allowed / present non-loopback default-deny; no OAuth PRM; audit records the authenticated actor on reset/delete/apply. Mandatory `TestMaildevScenarioCompat` (SendMail + 401 + Basic subject).
- Observability (`internal/observability`): slog JSON events with frozen names, hand-rolled OpenMetrics (no Prometheus client), catalog [`api/metrics/v1alpha1.json`](https://github.com/hilather/go-lab-maildev/blob/main/api/metrics/v1alpha1.json). Ready = SMTP bound + store initialized + management bound or explicitly off. `spec.observability.metrics.listen` (empty disables; default `127.0.0.1:9090`) and `publicPath` for authenticated `GET /v1/metrics`. `labmail healthcheck --url=…` probes ready.
- maildev 2.2.1 compat adapter (`internal/control/compat`) on the same management listener as `/v1` when `spec.listeners.management.compatEnabled` is true (default): `GET /email` JSON **array** (bodies omitted; `?skip=` and dotted filters), `GET /email/:id` marks read, `DELETE /email/:id` and `DELETE /email/all`, `GET /email/:id/html` (preview CSP), `GET /email/:id/attachment/:filename`, `GET /healthz`, redacted `GET /config` (`receiveOnly: true`). `POST /email/:id/relay` is always 403 `receive_only`. Documented deltas: ULID ids, sha256 attachment checksum, no `stream`.
- Native REST `/v1` (`internal/control/rest`) over `app.Service`: problem+json domain codes (`cursor_stale`, `store_over_new_cap` are first-class, not wrapped as `validation_failed`), HMAC list cursors, messages wait/extract (frozen RE2), preview CSP + `cid:` → `data:` rewrite, plan/apply JSON, health/ready, capability catalog, generated OpenAPI (`api/openapi/v1.json`) and capability manifest (`api/capabilities/v1.json`). `labmail serve` binds management HTTP from YAML (`--management-listen ADDR|off`). Native `GET /v1/messages/{id}` defaults `markRead=false`.
- Streamable HTTP MCP at `POST /mcp` (`internal/control/mcp`) over `app.Service`: official SDK `v1.7.0`, protocol `2026-07-28`, `mail_*` tools for every `PARITY_REQUIRED` row, `labmail://` resources, URI-only `subscriptions/listen` on `labmail://messages`, `labmail mcp-stdio --config … --token-file …`, `spec.management.mcp.allowLegacyClients` (default false), Origin missing-allowed / present non-loopback default-deny, generated `api/mcp/v1.json`, and `make test-parity`. Native `mail_message_get` defaults `markRead=false`. MCP is bearer-only.
- HTTP-less `internal/app.Service` with atomic config snapshot, plan/apply (coarse ops), reset that rereads YAML and wipes the inbox (Wipe is the only epoch bump; in-flight Insert → 451), `replaceStoreCaps` shrink rules, and an in-process audit ring on reset/delete/apply. SMTP insert stays on the data plane. `labmail serve` boots through `app.Service`; MAIL/RCPT/DATA re-read the live snapshot.
- Bounded memory inbox (`internal/store.Memory`): Crockford ULID ids, MIME extract via `internal/mimeparse` (the only `go-message` importer), stacked resident caps (raw + decoded), `fullPolicy: reject` → SMTP 452, single-message over `maxBytes` → 552, Wait + timeout, Wipe epoch (stale Insert → 451), optional tmpfs spill. Malformed MIME is still stored. `labmail serve` inserts into Memory, not Null.
- In-tree SMTP sink (`internal/smtp/{codec,server}`): greeting, HELO/EHLO, MAIL/RCPT/DATA/RSET/NOOP/QUIT/HELP, VRFY=252, EXPN=502, advertised SIZE/8BITMIME/SMTPUTF8/ENHANCEDSTATUSCODES, optional AUTH PLAIN/LOGIN (`smtp.auth.mode=plain_login`) and STARTTLS (`smtp.tls.mode=starttls`, optional or required). When STARTTLS is required, AUTH is withheld and rejected on cleartext so the lab password is never accepted before the handshake. Session and in-flight DATA caps. `labmail serve --config` binds SMTP. Interop: `net/smtp.SendMail` against localhost (default YAML still has no AUTH/TLS). Implicit SMTPS remains rejected.
- Fail-closed `labmail.dev/v1alpha1` YAML compiler: `KnownFields(true)`, reserved relay-key reject, default materialization, canonical revision hash, JSON Schema at `api/jsonschema/labmail.dev.v1alpha1.json`, and `labmail validate` / `canonicalize`.
- Repository foundation: Apache-2.0 license, Go 1.26 module `github.com/hilather/go-lab-maildev`, stub `labmail` CLI (`version` / `help` only), Makefile, fail-closed CI (format, lint, unit, docs), and the LabMail 1.0 design pack (`docs/01`–`13` + ADRs 0001–0007).
- Inbox list/get/delete/wait/extract exist on `app.Service`, native `/v1`, `mail_*` MCP tools, maildev `/email` compat, and the embedded operator UI.

### Changed

- None relative to a previous tag.

### Fixed

- Ready reports unready as soon as SMTP `Shutdown` begins (`Accepting()`), so `/v1/health/ready` is not 200 while the listener has stopped accepting. `GET /v1/status` `ready` follows the same probe when a Ready hook is installed.
- GA-001 review: `docs/11` serve/healthcheck progress line matches this tree; `docs/10` lists shipped compat/extract fixtures as present; fuzz corpus directories are fail-closed; tag-gate rejects empty `head_sha`.
- MCP inbox `resources/updated` is fan-out via the official SDK on both Streamable HTTP and `mcp-stdio` (URI-only `labmail://messages`). `subscriptions/listen` stays pinned to `2026-07-28` even when `allowLegacyClients` is true.
- Reset preflights store options (including a creatable spill directory) and installs new caps under one lock, so a failed reset cannot empty the inbox under the old snapshot.

### Removed or deprecated

- None.
