# SMTP semantics

Status: Proposed normative
Last reviewed: 2026-08-17
Related: [00-evaluation.md](00-evaluation.md), [ADR 0009](adr/0009-native-go-smtp.md), [ADR 0013](adr/0013-full-maildev-parity-and-comparison-lab.md)

## Role

LabMail is an **SMTP sink**. It speaks enough SMTP for MUAs and application libraries to submit mail. It does not route, retry, DSN, or deliver.

## Listen

| Setting | Default | MailDev flag / env |
| --- | --- | --- |
| Port | 1025 | `--smtp` / `MAILDEV_SMTP_PORT` |
| Bind | `::` (all) | `--ip` / `MAILDEV_IP` |
| Banner name | `labmail.local` (configurable) | not in MailDev; keep configurable for EHLO tests |

The lab publishes host 1025 → container 1025. Bind must not default to loopback ([mcp-integration-lab AGENTS rule 5](https://github.com/hilather/mcp-integration-lab/blob/main/AGENTS.md)).

## Session outline

1. 220 greeting.
2. EHLO/HELO.
3. Optional STARTTLS (if configured and not hidden).
4. Optional AUTH (if `incoming-user` set).
5. MAIL FROM.
6. One or more RCPT TO (all accepted if syntax OK).
7. DATA … terminator.
8. 250 queued as `<id>` (message is in the inbox, not a real queue).
9. RSET, NOOP, QUIT as usual.

No `TURN`, no `ETRN`, no outbound `ATR`.

## Advertised extensions

Always consider:

| Extension | Default | Hideable via `--hide-extensions` |
| --- | --- | --- |
| PIPELINING | on | yes |
| 8BITMIME | on | yes |
| SMTPUTF8 | on | yes |
| SIZE | on when max > 0 | hide SIZE only if we add it to the hide list; MailDev hide list is STARTTLS, PIPELINING, 8BITMIME, SMTPUTF8 |
| STARTTLS | only if TLS configured | yes |
| AUTH | only if incoming credentials configured | not in MailDev hide list |

Unknown hide names are config errors.

## AUTH

When `incoming-user` and `incoming-pass` (or password file) are set:

- Advertise `AUTH PLAIN LOGIN`.
- Reject MAIL FROM before successful AUTH with 530.
- Compare secrets in constant time.
- Do not log credentials.

When not set: anonymous submission allowed (lab default).

## TLS

- `--incoming-secure` + cert/key: implicit TLS listener **or** STARTTLS-only — MailDev uses smtp-server `secure`. Match: if secure, wrap the listener in TLS (implicit). If not secure but certs present, prefer STARTTLS. Document the chosen matrix in examples and tests.
- Lab default: plain SMTP, STARTTLS not required (labinfo connection block).

## Size and resource limits

| Limit | Default | On exceed |
| --- | --- | --- |
| Max message bytes | 50 MiB | SMTP 552, not stored |
| Max recipients per transaction | 100 | 452 after cap |
| Concurrent SMTP sessions | 64 (config) | 421 |
| Inbox cardinality | 10_000 | 452 on MAIL/DATA when full |
| Line length | RFC-reasonable bound | 500/554, not stored |

No silent eviction of old mail to accept new mail (differs from MailDev memory `maxEmails` shift). Fail closed. Operators raise the cap or reset.

## Internationalization

If SMTPUTF8 is advertised, accept UTF-8 in envelope and headers. If hidden, reject non-ASCII envelopes with 553/550 as appropriate. Tests for both.

## IDs in the 250 reply

Include the 8-char inbox id in the 250 text (`250 2.0.0 Ok: queued as XwgKAxto`) so tests can correlate without REST. REST id equals that id.

## VRFY / EXPN

Respond **502** (not implemented). This is a sink, not a directory.

## Receive-only enforcement at this layer

`internal/smtpd` is ingest-only. Outbound lives in `internal/relay`, invoked from `app` on REST/MCP relay and on auto-relay after ingest. Comparison-lab tests prove both.

## Error mapping (informative)

| Condition | SMTP | domainerr |
| --- | --- | --- |
| Unauthenticated (AUTH required) | 530 | `unauthenticated` |
| Message too large | 552 | `payload_too_large` |
| Inbox full | 452 | `resource_exhausted` |
| Syntax | 501 | `invalid_argument` |
| Shutdown | 421 | `unavailable` |
