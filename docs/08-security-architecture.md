# Security Architecture

Status: Proposed normative behavior
Owners: Security, SMTP, Control Plane
Last reviewed: 2026-08-18 (SEC-001 + DEP-001 + UI-001)
Related ADRs: 0002, 0003, 0005, 0007

LabMail is a lab sink, not a public MX. The critical invariant is receive-only: outbound SMTP must be unrepresentable.

## Threat model (lab, not public MX)

| Threat | Severity | Mitigation |
|---|---|---|
| Open relay / LabMail used to spam | **Critical** | No SMTP client; reserved YAML keys; 403 relay; import boundary; tests |
| Unauthenticated inbox read on a published `:1080` | High | Default YAML is `bearer_and_basic`; SEC-001 smoke asserts 401; no `dev-loopback-unauth` in the image default |
| HTML mail XSS in the operator browser | High | Preview CSP `img-src data:` only; `cid:` inlined as `data:` (not HTTP); iframe `sandbox` without scripts/same-origin; no parent `innerHTML`; download route `Content-Disposition: attachment` |
| Path traversal via attachment filename | High | Ignore client path; server-side `attId`; sanitize download name |
| SMTP AUTH / management secret leakage | High | File refs only; redaction in logs/export/audit/MCP; never log DATA by default (`logMailContents` is not offered — maildev’s `--log-mail-contents` is a footgun) |
| Store memory DoS | Medium | Caps + reject-full + DATA size + session caps |
| SMTP command injection / smuggling | Medium | Line limits; no PIPELINING; strict CRLF DATA end; fuzz codec |
| DNS rebinding on management | Medium | Present non-loopback Origin default-deny; missing Origin allowed |
| Confused deputy via Basic vs Bearer | Low | Both map to one `tokenRef` principal and one scope set |
| Supply chain | Medium | Pin modules and Actions SHAs; govulncheck; SBOM on release |

## Receive-only enforcement (hard invariant)

Defense in depth:

1. **Schema.** Reserved key names rejected. No type exists for an outgoing host.
2. **Binary.** Production `internal/smtp`, `internal/store`, and `internal/app` must not import `net/smtp` and must not call `net.Dial`, `net.DialTimeout`, or `net.Dialer.Dial` **at all** (Listen/Accept only). Test helper `internal/smtptest` is allowed in `*_test.go` only.
3. **Import boundary** (`internal/smtp/import_test.go`): `internal/smtp` may import `internal/model`, `internal/store`, `internal/mimeparse`, `internal/observability`. It may **not** import `internal/control`, `net/http`, or `net/smtp`. Static check fails on any `Dial` ident in `internal/smtp`, `internal/store`, and `internal/app`.
4. **HTTP.** `POST /email/{id}/relay` and any `/v1/**/relay` → 403 `receive_only`.
5. **UI.** No control that implies send.
6. **Tests.** Table-driven: every reserved YAML key; compat relay; `go list` / static check that no production `.go` file references `outgoing-host` as a feature; SMTP session cannot be configured to connect outbound.
7. **mcp-integration-lab.** After the swap, `internal/maildev` either dies or becomes a LabMail YAML renderer that still rejects relay keys. Keep a regression test.

Reserved YAML names (normalize strips dashes/underscores/case):

```
outgoing, outgoingHost, outgoing-host, outgoingPort, outgoingUser,
outgoingPass, outgoingSecure, autoRelay, auto-relay, autoRelayRules,
auto-relay-rules, relay, smarthost, smartHost, forwardTo, mx, deliver
```

## Authn/z details

- Tokens: **≥256 bits** entropy (TacLab ADR 0010), compared as SHA-256 digests in the in-memory index. Bootstrap file is the only durable secret.
- Basic: `username` exact match + constant-time password compare; then the principal is `tokens[basic.tokenRef]`. Failed Basic and failed Bearer both return `401` `unauthenticated` with `WWW-Authenticate: Bearer realm="labmail"` **and** (if basic enabled) `WWW-Authenticate: Basic realm="labmail"`.
- UI session cookie name **`labmail_session`**: `HttpOnly`, `SameSite=Lax`, `Secure` iff management TLS; CSRF header `X-LabMail-CSRF` required on cookie-authenticated mutations even over HTTP (`POST /v1/session`, `DELETE /v1/session`). `GET /v1/session` returns the CSRF secret for a valid cookie (reload recovery). Session JSON (and other REST JSON) is `Cache-Control: no-store`. Native `GET /v1/messages/{id}` defaults `markRead=false`; the SPA does not pass `markRead=true` (compat `GET /email/:id` still marks read). Token files are reread on reset and apply; the session table is cleared only when the compiled auth identity changes. A failed secret reread keeps the previous verifier and live sessions.
- No `.well-known/oauth-protected-resource` (ADR 0005: lab static bearer).
- `X-Forwarded-For` is not trusted.
- No CORS headers. OPTIONS is not a success path.
- Default container YAML is `bearer_and_basic`. `dev-loopback-unauth` is not the image default.

Scopes and roles: [docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md).

## HTML preview / XSS

`GET /v1/messages/{id}/preview` response headers (frozen):

```
Content-Type: text/html; charset=utf-8
Content-Security-Policy: default-src 'none'; img-src data:; style-src 'unsafe-inline'
X-Content-Type-Options: nosniff
Content-Disposition: inline
```

UI iframe: `<iframe src="/v1/messages/{id}/preview" sandbox>` — **no** `allow-scripts`, **no** `allow-same-origin`, **no** `allow-popups-to-escape-sandbox`. Not `srcdoc`. Never parent `innerHTML`.

**`cid:` rewrite:** the preview document inlines matching parts as `data:<contentType>;base64,…` URLs before it is served. Do **not** rewrite `cid:` to HTTP attachment paths. Parts larger than 2 MiB decoded, or missing `Content-ID`, are omitted. Remote `http(s):` images stay broken (no tracking pixels).

Attachments download: `Content-Disposition: attachment` + `X-Content-Type-Options: nosniff`. Never use the client-supplied filename as a filesystem path.

## Data handling

- Inbox contents are lab data and may contain credentials (password resets). Treat as secret-adjacent: do not put subjects or addresses in metric labels; audit “mail.received” records id, size, recipient **count**, not the body.
- Export of config never includes token values or SMTP passwords.
- `GET` raw message is authorized `mail.read` and is the whole RFC 822 bytes — operators must understand that.

## Container

Non-root UID 65532, read-only root, no caps, no-new-privileges, no shell, no Docker socket, no writable volume except tmpfs `/tmp` (and optional spill under `/tmp/labmail-spill`).
