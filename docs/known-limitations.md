# Known limitations (1.0 / v1.0.0-rc.3)

Honest residual for LabMail 1.0, last reviewed against this tree’s **v1.0.0-rc.3** notes. These are not defects hidden from the notes. They are out-of-scope product bounds, documented deltas versus maildev 2.2.1, or work that is **not** claimed here.

Last reviewed: 2026-08-20 (v1.0.0-rc.3)

This file is the operator-facing residual list. The numbered pack still wins on conflict: [docs/01-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/01-architecture.md#residual-limitations-10). Release notes: [docs/releases/v1.0.0-rc.3.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.3.md).

LabMail is a **lab sink**. It is **not** a public MTA and does **not** claim RFC 5321/5322 completeness.

## This tree versus what is not tagged

Design PRs 1–14, the side-by-side probe, `spec.smtp.behavior`, and originAllowlist sentinels are in this tree. What is **not** this candidate:

| In this tree | Not this tag |
|---|---|
| SMTP AUTH/STARTTLS, OBS, DEP files, SWAP examples, inbox SPA, GA notes, QA handshake scripting | Published `ghcr.io/hilather/labmail` digest |
| First-party inbox SPA | MailDev Angular / WebSocket UI |
| Overlay examples in this repo | mcp-integration-lab compose/image pin |

**GA / 1.0 is not this rc.** The inbox UI is present (Q2). That does not make this SHA a 1.0 GA tag.

## Not a public MTA

- No DSN, CHUNKING, advertised PIPELINING, Sieve, per-recipient quotas, or greylisting.
- Interop target is common lab clients (`net/smtp`, nodemailer, Django, Spring, swaks with STARTTLS off). Not Internet MX hosting.
- `VRFY` defaults to `252`. `EXPN` / `BDAT` / `ETRN` / `ATRN` / `TURN` are `502`. A QA `replies.vrfy` override is operator-opt-in.
- Open-RCPT capture is **not** relay. There is no SMTP client in production packages.

## SMTP (this tree)

- Default lab posture: no AUTH, no TLS required, any MAIL FROM / RCPT TO accepted, SIZE advertised.
- AUTH PLAIN/LOGIN and STARTTLS are YAML-optional and implemented. Implicit SMTPS stays a validate reject.
- Implicit SMTPS (`smtp.tls.mode: implicit`, maildev `--incoming-secure`) is **1.1**. 1.0 validate rejects it and does not silently map to STARTTLS.
- Default `maxMessageBytes` is **10 MiB** (maildev implicit ~50 MiB; 2.2.1 has no `--max-message-size` flag).
- Optional `spec.smtp.behavior` can script greeting/command delays (≤30s), reply overrides (`CODE text`), drop-on-connect, and close-after-verb. It is default-off and deterministic — not a random chaos engine (D16). Live apply is `replaceSMTPBehavior` (next command).

## Store and MIME

- MIME parse of pathological messages may yield empty text/html with `parseWarning`; raw is still stored.
- Worst-case RSS ≈ stored `maxBytes` (256 MiB) + `maxInFlightDataBytes` (64 MiB) + ~64 MiB slack ≈ **384 MiB**. Caps are stacked: in-flight does not reduce inbox capacity.
- Spill on tmpfs does not add a second disk budget — it is still RAM. Spill is wiped on reset/restart. There is no mail-directory.
- Single replica; no shared inbox.
- Default soak in CI accepts **8** messages, Waits, and Wipes. Raise with `-soak-n` / `LABMAIL_SOAK_N`. Absolute QPS/p99 are not CI gates.

## Compat versus maildev 2.2.1

- Compat `/email` ids are Crockford ULIDs, not 8-char maildev ids.
- `GET /email` list omits `text`/`html` (bodies empty); checksum is sha256 not md5; no leaked `stream`.
- `GET /config` is a redacted LabMail shape (`receiveOnly: true`), not a clone of 2.2.1 internals.
- Compat does not implement maildev WebSocket.
- `POST /email/:id/relay` never works (intentional `403` `receive_only`).
- `mail-directory` and `base-pathname` are rejected (no passthrough).
- `ui.enabled: false` hides the SPA only; REST/MCP stay up (not maildev `--disable-web`).

## Origin policy / inbox UI

- Empty `originAllowlist` still **403s non-loopback SPA JS** (`forbidden` / `origin is not allowed`) until the operator lists an exact Origin, `"private"`, or `"*"`. `GET /` HTML is often 200 (no Origin); hashed module scripts send Origin.
- `"*"` turns off DNS-rebinding origin defense for all http(s) Origins. Keep `bearer_and_basic`. Do not ship `"*"` in the image default.
- `"private"` follows Go `net.IP.IsPrivate()` (RFC 1918 + RFC 4193 ULA). RFC 6598 CGNAT / Tailscale `100.x` is **not** private; use exact or `"*"`.
- No CORS headers. `OPTIONS` stays `403` `CORS is disabled` even with hatches on. Remote Vite without a proxy is not 1.0.
- Cookbook: [docs/11-deployment.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/11-deployment.md#origin-allowlist-cookbook).

## Control plane

- MCP clients requiring OAuth PRM cannot authorize. MCPJungle needs `allowLegacyClients: true` (D17).
- MCP protocol is **2026-07-28**. `mcp-stdio` is a developer adapter, not an image entrypoint.
- HTML preview blocks remote `https:` images (no tracking pixels). `cid:` is inlined as `data:` only; parts larger than 2 MiB decoded are omitted.
- Catalog service id remains **`maildev`** during the swap release (D15 / Q1). Rename only in a later mcp-integration-lab release.

## Deployment

- Healthcheck plane is HTTP `/v1/health/ready` (not SMTP/`node`). Ready still requires SMTP bound.
- Dockerfile and `make test-container` are in-tree. This candidate does not publish a `ghcr.io/hilather/labmail` digest, SBOM, or provenance.
- Application binaries built without ldflags report version `dev`. The notes version `1.0.0-rc.3` is the candidate identity for the tag, not the default `dev` string.
- Required GitHub Actions green-on-tag is enforced by Release `tag-gate`.

## Explicit non-goals (unchanged)

Outbound SMTP, relay, smarthost, DSN generation, MX lookup, being an MDA / IMAP / POP3 server, full public-MTA conformance, CHUNKING, advertised PIPELINING, implicit SMTPS in 1.0, durable mail-directory, multi-replica inbox, maildev WebSocket / Angular UI / v3 `/api`, OAuth PRM, DKIM/SPF/DMARC, virus scanning, wrapping the Node maildev image, and a LabDNS-style random chaos engine (deterministic `spec.smtp.behavior` is the 1.0 QA knob).
