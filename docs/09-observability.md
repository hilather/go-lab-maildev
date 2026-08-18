# Observability

Status: Proposed normative behavior
Owners: Observability, SMTP, Control Plane
Last reviewed: 2026-08-17 (OBS-001)
Related ADRs: 0001

## Logs (`log/slog` JSON)

Frozen event names in [`api/metrics/v1alpha1.json`](https://github.com/hilather/go-lab-maildev/blob/main/api/metrics/v1alpha1.json) (`labmail.dev/metrics/v1alpha1`; generated from `internal/observability`):

```
smtp.accepted smtp.rejected smtp.session_end
store.inserted store.deleted store.wiped store.full
http.request mcp.call
auth.failure auth.success
state.reset state.apply
```

Fields: `timestamp`, `level`, `event`, `component`, `request_id`, `message_id`, `smtp_code`, `capability`, `result`, `error_code`, `duration_ms`, `store_generation`. Do **not** log raw DATA, passwords, or `Authorization`. Remote IP only at `debug`.

## Metrics (hand-rolled OpenMetrics)

Same exposition style as LabDNS `internal/observability`: write OpenMetrics text; **do not** import `github.com/prometheus/*`. Go source of truth: `internal/observability`. `make generate` / `make verify-generated` keep [`api/metrics/v1alpha1.json`](https://github.com/hilather/go-lab-maildev/blob/main/api/metrics/v1alpha1.json) current. `spec.observability.metrics.listen` default `127.0.0.1:9090` (empty disables). A lab overlay that needs compose scraping sets `listen: ":9090"` (or `0.0.0.0:9090`). `publicPath: true` exposes authenticated `GET /v1/metrics` on the management listener; default `false`. The scrape listener serves `/metrics` unauthenticated (bind loopback unless the overlay needs compose scraping).

Bounded labels only. Do not put subjects or addresses in metric labels.

| Name | Kind | Labels |
|---|---|---|
| `labmail_smtp_sessions_total` | counter | `result` (`ok`, `rejected`, `timeout`) |
| `labmail_smtp_messages_total` | counter | `result` (`accepted`, `too_large`, `store_full`, `auth`, `tls`) |
| `labmail_smtp_session_duration_seconds` | histogram | — |
| `labmail_smtp_sessions_active` | gauge | — |
| `labmail_store_messages` | gauge | — |
| `labmail_store_bytes` | gauge | — |
| `labmail_store_evictions_total` | counter | — |
| `labmail_store_waiters` | gauge | — |
| `labmail_http_requests_total` | counter | `capability`, `code_class` |
| `labmail_http_request_duration_seconds` | histogram | `capability` |
| `labmail_mcp_calls_total` | counter | `tool`, `result` |
| `labmail_auth_failures_total` | counter | `reason` |
| `labmail_audit_events_total` | counter | `event` |
| `labmail_telemetry_dropped_total` | counter | `reason` |

## Health

| Probe | Meaning |
|---|---|
| `GET /v1/health/live` | Process up (listener goroutines not deadlocked) |
| `GET /v1/health/ready` | SMTP bound **and** (management bound or explicitly off) **and** store initialized |
| `GET /healthz` | Compat alias of ready |

Ready does **not** require MCP clients or a non-empty inbox.

Healthcheck CLI: `labmail healthcheck --url=http://127.0.0.1:1080/v1/health/ready`.

The integration-lab compose healthcheck plane changes from SMTP TCP (`node -e connect(1025)`) to this HTTP ready probe (ready still requires SMTP bound).

## Alerting (operator, not shipped as SaaS)

- Ready failing for >30s
- `store_full` rate > 0 in a lab that expected capture
- `auth_failures` spike (token mismatch after swap)
