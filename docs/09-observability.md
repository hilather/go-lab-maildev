# Observability

Status: Proposed
Last reviewed: 2026-08-17

## Logs

`log/slog` JSON. Fields: `msg`, `level`, `component` (`smtpd|rest|mcp|store|app`), `email_id`, `remote`, `err`. No secrets. Optional `logMailContents` adds `subject` and body length; full body only when flag set.

## Metrics (GA minimum)

Prometheus on the management listener (e.g. `GET /metrics`) behind the same auth as REST **or** localhost-only — pick in wave 4 and document. Suggested series:

- `labmail_smtp_sessions_total`
- `labmail_smtp_messages_accepted_total`
- `labmail_smtp_messages_rejected_total{reason}`
- `labmail_inbox_messages`
- `labmail_inbox_bytes`
- `labmail_http_requests_total{code,route}`
- `labmail_mcp_calls_total{tool,code}`

## Health

| Endpoint | Meaning |
| --- | --- |
| `/v1/health/live` | Process up |
| `/v1/health/ready` | SMTP and HTTP accept running |
| `/healthz` | MailDev alias of ready (JSON `true`) |

CLI `labmaild healthcheck` used as Compose `HEALTHCHECK`.

## Tracing

Optional later (OTel). Not GA-required.
