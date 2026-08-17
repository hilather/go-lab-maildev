# ADR 0002: In-tree SMTP server and structural receive-only

Status: Accepted
Date: 2026-08-17
Decisions: D7, D8

## Context

Today’s receive-only guarantee in mcp-integration-lab is a compose-time flag filter in `internal/maildev` that rejects `outgoing-*` and `auto-relay*`. A profile or a future maildev flag can reintroduce relay if that guard is bypassed. LabMail must make outbound SMTP *unrepresentable*.

Family appliances own their protocol state machines (TacLab ADR 0007, LabDNS `dnswire` isolation). Using `emersion/go-smtp` would pull in a server that has relay/outbound hooks.

## Decision

**D7 — Receive-only is structural.**

- No SMTP client in production packages.
- Config loader rejects any key matching `outgoing*`, `auto-relay*`, `relay*`, `smarthost*` after normalize (strip dashes, underscores, and case).
- Compat `POST /email/{id}/relay` (and any `/v1/**/relay`) returns 403 `receive_only`.
- Import-boundary test: production `internal/smtp`, `internal/store`, and `internal/app` must not import `net/smtp` and must not call `Dial` / `DialTimeout` / `Dialer.Dial` at all. Listen/Accept only.
- Test helper `internal/smtptest` is allowed in `*_test.go` only.

**D8 — In-tree SMTP server.**

- Implementation lives in `internal/smtp/{codec,server}`.
- Standard library `net` + `crypto/tls` only.
- No `emersion/go-smtp` in the server.
- Implicit SMTPS (`smtp.tls.mode: implicit`) is **1.1** and rejected at 1.0 validate.

Review trigger: if PR 3a interop is still red at rc, write an ADR to vendor `emersion/go-smtp` *behind* `internal/smtp` with relay hooks compiled out — not a product-level SMTP client.

## Consequences

- Every command and reply is listed in [docs/02-smtp-semantics.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/02-smtp-semantics.md).
- Receive-only can be proven with schema + import-boundary + HTTP 403 + UI absence.
- More session code than a library wrap.
- Common clients (`net/smtp`, nodemailer, Django, Spring, swaks) are the interop target, not a public MTA.

## Alternatives considered

- Use `emersion/go-smtp` for the server: less session code, but harder to prove “no outbound, no relay hook”. Rejected for 1.0.
- Keep wrapping maildev and add a sidecar MCP: receive-only stays a flag filter; two processes; Node image. Rejected.
- Implicit SMTPS in 1.0: maildev `--incoming-secure` is TLS-on-accept, not STARTTLS. Silently mapping would lie. Rejected; shim rejects those flags.

## Review triggers

Review this decision when PR 3a interop is still red at rc, a requirement for implicit SMTPS on `:1025` appears (must not share cleartext), or a new outbound path is proposed.
