# Security architecture

Status: Proposed normative
Last reviewed: 2026-08-17
Related ADRs: 0002, 0007, 0011

## Trust boundaries

```text
[untrusted SMTP clients] --SMTP--> smtpd (data plane)
[untrusted HTTP clients] --HTTP--> authn --> authz --> app
[browser] --same origin--> UI + /ws (after HTTP auth)
[MCPJungle] --bearer--> /mcp
```

Captured mail is **untrusted content**. It may contain HTML, SVG, scripts, tracking pixels, and executable attachments. The sanitizer and Content-Disposition are controls, not a sandbox for operators who download `.eml` files.

## Authentication

1. **HTTP basic** — MailDev/lab. Username/password (password from file or env). Constant-time compare.
2. **Bearer token** — lab appliance family / MCPJungle. Token from file. Constant-time compare.
3. Either may be enabled. If **both** enabled, a request succeeds if one succeeds.
4. If **neither** enabled: refuse to start unless `--insecure-no-auth` is set (dev only, log a warning). Lab always enables basic today; cutover should add bearer for MCP without removing basic.

SMTP AUTH is independent (data plane).

## Authorization

See scopes in [05-control-plane-and-parity.md](05-control-plane-and-parity.md). Basic auth ⇒ all scopes. Bearer ⇒ configured scopes (default all three for the single lab token).

## Secrets

Never log: basic password, bearer token, SMTP AUTH password, `Authorization` header, query tokens (query tokens are forbidden).

`GET config` redacts credentials. `logMailContents` logs parsed mail bodies, **not** AUTH.

## HTML and attachments

- Sanitize HTML before preview (`internal/sanitize`).
- Serve HTML preview with `Content-Type: text/html; charset=utf-8` and restrictive `Content-Security-Policy` (no `unsafe-inline` scripts; allow inline styles if email needs them — document the CSP in wave 3).
- Attachments: `Content-Disposition: attachment` except rewritten CID images in HTML.
- Do not execute attachments.

## Process hardening

Non-root, read-only rootfs, `cap_drop: ALL`, `no-new-privileges`, no Docker socket, no outbound SMTP. Optional HTTP/SMTP TLS with operator-provided certs.

## Abuse

Bounded message size, connections, recipients, inbox cardinality, wait timeout, MCP binary size. Slowloris: HTTP timeouts.

## Audit

Structured events: ingest, delete, reset, auth failure (no secret). In-memory ring optional; stdout JSON is enough for GA.

## What we do not claim

This is a **lab sink**. Anyone who can SMTP to it can plant phishing HTML for whoever opens the UI. Anyone with management credentials can read all captured mail (which may include password-reset links). Bind accordingly; lab publishes on all interfaces **on purpose** behind lab tokens.
