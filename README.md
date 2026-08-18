# LabMail

**Receive-only SMTP lab appliance** in the LabDNS / LabLDAP / TacLab family.

Systems under test deliver RFC 5321 SMTP here. LabMail captures, indexes, and exposes every accepted message over REST, MCP, and an embedded inbox UI. It **never** opens an outbound SMTP session, **never** relays, and **never** implements `POST /email/:id/relay`. Desired state is a fail-closed `labmail.dev/v1alpha1` YAML file. Captured mail is ephemeral: restart or reset returns the process to the mounted bootstrap and an empty inbox.

[![CI](https://img.shields.io/github/actions/workflow/status/hilather/go-lab-maildev/ci.yml?branch=main&label=CI)](https://github.com/hilather/go-lab-maildev/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/hilather/go-lab-maildev?label=Go)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](https://github.com/hilather/go-lab-maildev/blob/main/LICENSE)

Status: **v1.0.0-rc.2 candidate** (rc.1 plus deterministic `spec.smtp.behavior` QA handshake scripting). The `labmail` binary implements `version`, `help`, `validate`, `canonicalize`, `healthcheck`, `mcp-stdio`, and `serve` (SMTP plus `/v1`, `POST /mcp`, `/email`, slog JSON, OpenMetrics, and the SPA at `/`). LabMail is a lab sink, **not** a public MTA. Residuals: [docs/known-limitations.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/known-limitations.md). Notes: [docs/releases/v1.0.0-rc.2.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.2.md).

Module [`github.com/hilather/go-lab-maildev`](https://github.com/hilather/go-lab-maildev) · Binary `labmail` · Image `ghcr.io/hilather/labmail` · YAML `apiVersion: labmail.dev/v1alpha1`, `kind: LabMail`

New here? Start with [START-HERE.md](https://github.com/hilather/go-lab-maildev/blob/main/START-HERE.md). Architecture, ADRs, and the program board are indexed in [Documentation](#documentation).

This repository will replace the off-the-shelf [maildev](https://github.com/maildev/maildev) image used by [mcp-integration-lab](https://github.com/hilather/mcp-integration-lab). Family siblings:

- [LabDNS](https://github.com/hilather/go-lab-dns)
- [LabLDAP](https://github.com/hilather/go-lab-ldap-mcp)
- [TacLab](https://github.com/hilather/go-lab-tacacs-mcp)

## Intended lab role

The integration lab currently publishes maildev as:

| Plane | Default host port | Role |
|---|---|---|
| SMTP ingest | 1025 | outbound SMTP target for systems under test |
| Management / UI / REST | 1080 | inspect captured mail (native `/v1`, `/email` compat, and `POST /mcp`) |

Those listeners, the receive-only posture, wipe-on-restart semantics, and HTTP Basic on `/email` are the swap contract. During the swap release the labinfo catalog id stays **`maildev`**.

## Quick start

```bash
git clone https://github.com/hilather/go-lab-maildev.git
cd go-lab-maildev
go version   # go1.26.x
go build -o bin/labmail ./cmd/labmail
./bin/labmail version
./bin/labmail help
./bin/labmail validate --config testdata/config/valid/defaults.yaml
./bin/labmail serve --config testdata/config/valid/defaults.yaml --smtp-listen 127.0.0.1:1025 --management-listen 127.0.0.1:1080
```

`serve` binds SMTP from the compiled YAML (override with `--smtp-listen`) and management HTTP from `spec.listeners.management.address` (override with `--management-listen ADDR|off`). `--shutdown-timeout` (default 5s) and `--pid-file` are optional. Native `/v1`, the inbox SPA at `/`, maildev `/email` (when `compatEnabled`, default true), and Streamable HTTP `POST /mcp` share that listener; `POST /email/:id/relay` is 403. Accepted messages are parsed and stored in a bounded memory inbox (ULID ids, stacked caps, Wipe on shutdown). Ready is SMTP bound + store up. Probe readiness with `labmail healthcheck --url=http://127.0.0.1:1080/v1/health/ready` or `GET /healthz`. Metrics are hand-rolled OpenMetrics on `spec.observability.metrics.listen` (default `127.0.0.1:9090`; empty disables); `publicPath: true` also serves `GET /v1/metrics`. Developer MCP: `labmail mcp-stdio --config testdata/config/valid/defaults.yaml --token-file /path/to/token` (required unless `auth.mode` is `dev-loopback-unauth`). Production UI assets: `make web-build` (Node **22.14.0**) copies `web/dist` into `internal/web/dist`.

Hardened image: non-root UID `65532`, scratch, read-only root, `cap_drop: ALL`. Healthcheck is HTTP ready (not SMTP/`node`). Compose smoke: [examples/compose.smoke.yaml](https://github.com/hilather/go-lab-maildev/blob/main/examples/compose.smoke.yaml).

## Build and test

```bash
make format
make lint
make generate
make verify-generated
make test
make test-race
make test-fuzz-smoke
make test-parity
make test-config-compat
make test-docs
make test-container
make test-changelog
make web-test
make web-build
make build
```

Required CI jobs: format, lint, unit, race, fuzz-smoke, generated-file, documentation, security-scan, changelog, parity, config-compat, container-test, web. There is no optional or bypassable job. `make test-container` needs Docker. A `v*` tag is refused unless Release `tag-gate` sees those jobs green on the exact SHA.

## Documentation

The numbered pack is normative after FND-001. Cross-file links are absolute.

| Path | Topic |
|---|---|
| [START-HERE.md](https://github.com/hilather/go-lab-maildev/blob/main/START-HERE.md) | Onboarding |
| [AGENTS.md](https://github.com/hilather/go-lab-maildev/blob/main/AGENTS.md) | Contributor / agent rules |
| [docs/README.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/README.md) | Pack index |
| [docs/01-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/01-architecture.md) | Process and package model |
| [docs/02-smtp-semantics.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/02-smtp-semantics.md) | SMTP command table, limits, AUTH/TLS |
| [docs/03-message-store.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/03-message-store.md) | Caps, wait, wipe, spill |
| [docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md) | YAML, revisions, reset |
| [docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md) | Capability registry |
| [docs/06-rest-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/06-rest-api.md) | Native `/v1` |
| [docs/07-mcp-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/07-mcp-api.md) | MCP tools and resources |
| [docs/08-security-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/08-security-architecture.md) | Auth, XSS, receive-only |
| [docs/09-observability.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/09-observability.md) | Logs, metrics, probes |
| [docs/10-testing-strategy.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/10-testing-strategy.md) | Test layers |
| [docs/11-deployment.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/11-deployment.md) | Image, compose, CLI |
| [docs/12-maildev-compat.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/12-maildev-compat.md) | `/email` mapping |
| [docs/13-integration-lab-swap.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/13-integration-lab-swap.md) | mcp-integration-lab swap |
| [docs/known-limitations.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/known-limitations.md) | 1.0 residuals (not a public MTA) |
| [docs/releases/v1.0.0-rc.2.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.2.md) | rc.2 notes; tag only on green CI |
| [tasks/00-program-board.md](https://github.com/hilather/go-lab-maildev/blob/main/tasks/00-program-board.md) | PRs 1–14 |
| [CHANGELOG.md](https://github.com/hilather/go-lab-maildev/blob/main/CHANGELOG.md) | Curated history |

## License

[Apache License 2.0](https://github.com/hilather/go-lab-maildev/blob/main/LICENSE)
