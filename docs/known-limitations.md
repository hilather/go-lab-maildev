# Known limitations (1.0 / v1.0.0-rc.1)

Honest residual for LabMail 1.0, last reviewed against this tree’s **v1.0.0-rc.1** notes. These are not defects hidden from the notes. They are out-of-scope product bounds, documented deltas versus maildev 2.2.1, or work that lives on sibling branches and is **not** claimed here.

Last reviewed: 2026-08-18 (GA-001)

This file is the operator-facing residual list. The numbered pack still wins on conflict: [docs/01-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/01-architecture.md#residual-limitations-10). Release notes: [docs/releases/v1.0.0-rc.1.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.1.md).

LabMail is a **lab sink**. It is **not** a public MTA and does **not** claim RFC 5321/5322 completeness.

## This tree versus stacked siblings

This branch stacks through UI-001 (inbox SPA), SEC-001, API/MCP/compat, STORE, SMTP-001a, and CFG. The following land on sibling branches and are **not** in this tree. Do not treat them as GA here:

| Sibling | ID | What is missing here |
|---|---|---|
| SMTP AUTH / STARTTLS | SMTP-001b | `serve`, live apply, and reset **fail-close** `smtp.auth.mode != none` and `smtp.tls.mode != off`. No PLAIN/LOGIN, no STARTTLS. |
| Observability | OBS-001 | Frozen slog event names and the hand-rolled OpenMetrics listener are not implemented. Ready is HTTP `/v1/health/ready`; there is no `:9090` catalog yet. |
| Hardened image | DEP-001 | No `Dockerfile`, no `ghcr.io/hilather/labmail`, no `scripts/test-container.sh`. `make test-container` stays fail-closed. |
| Swap overlay examples | SWAP-001 | `docs/13` is the design BOM from FND-001. Example `labmail.yaml` / MCPJungle server JSON for mcp-integration-lab are not in this tree. |

Q2 is closed: **GA / 1.0 is not done without the inbox UI**. The SPA **is** on this branch, so this rc.1 candidate includes it. That does not make this SHA a 1.0 GA tag.

## Not a public MTA

- No DSN, CHUNKING, advertised PIPELINING, Sieve, per-recipient quotas, or greylisting.
- Interop target is common lab clients (`net/smtp`, nodemailer, Django, Spring, swaks with STARTTLS off). Not Internet MX hosting.
- `VRFY` is always `252`. `EXPN` / `BDAT` / `ETRN` / `ATRN` / `TURN` are `502`.
- Open-RCPT capture is **not** relay. There is no SMTP client in production packages.

## SMTP (this tree)

- Default lab posture: no AUTH, no TLS required, any MAIL FROM / RCPT TO accepted, SIZE advertised.
- AUTH PLAIN/LOGIN and STARTTLS are YAML-optional in the **schema** but **not implemented** until SMTP-001b. Asking for them fail-closes.
- Implicit SMTPS (`smtp.tls.mode: implicit`, maildev `--incoming-secure`) is **1.1**. 1.0 validate rejects it and does not silently map to STARTTLS.
- Default `maxMessageBytes` is **10 MiB** (maildev implicit ~50 MiB; 2.2.1 has no `--max-message-size` flag).
- No SMTP chaos / fault injection (D16).

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

## Control plane

- MCP clients requiring OAuth PRM cannot authorize. MCPJungle needs `allowLegacyClients: true` (D17).
- MCP protocol is **2026-07-28**. `mcp-stdio` is a developer adapter, not an image entrypoint.
- HTML preview blocks remote `https:` images (no tracking pixels). `cid:` is inlined as `data:` only; parts larger than 2 MiB decoded are omitted.
- Catalog service id remains **`maildev`** during the swap release (D15 / Q1). Rename only in a later mcp-integration-lab release.

## Deployment (not in this tree)

- Healthcheck plane in compose **will** change from SMTP TCP (`node`) to HTTP `/v1/health/ready` when DEP-001 lands (ready still requires SMTP bound).
- No tag, image digest, SBOM, or provenance in this candidate. `v1.0.0-rc.1` notes ship before that tag exists.
- Application binaries built without ldflags report version `dev`. The notes version `1.0.0-rc.1` is the candidate identity for the tag, not the default `dev` string.
- Required GitHub Actions green-on-tag is enforced by Release `tag-gate`, not by this branch commit alone.

## Explicit non-goals (unchanged)

Outbound SMTP, relay, smarthost, DSN generation, MX lookup, being an MDA / IMAP / POP3 server, full public-MTA conformance, CHUNKING, advertised PIPELINING, implicit SMTPS in 1.0, durable mail-directory, multi-replica inbox, maildev WebSocket / Angular UI / v3 `/api`, OAuth PRM, DKIM/SPF/DMARC, virus scanning, wrapping the Node maildev image, and a LabDNS-style chaos engine.
