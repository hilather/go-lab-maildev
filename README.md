# LabMail

**Receive-only SMTP lab appliance** in the LabDNS / LabLDAP / TacLab family.

Systems under test deliver RFC 5321 SMTP here. LabMail will capture, index, and expose every accepted message over REST, MCP, and an embedded inbox UI. It **never** opens an outbound SMTP session, **never** relays, and **never** implements `POST /email/:id/relay`. Desired state is a fail-closed `labmail.dev/v1alpha1` YAML file. Captured mail is ephemeral: restart or reset returns the process to the mounted bootstrap and an empty inbox.

[![CI](https://img.shields.io/github/actions/workflow/status/hilather/go-lab-maildev/ci.yml?branch=main&label=CI)](https://github.com/hilather/go-lab-maildev/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/hilather/go-lab-maildev?label=Go)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](https://github.com/hilather/go-lab-maildev/blob/main/LICENSE)

Status: **foundation + fail-closed YAML + plain SMTP sink**. The `labmail` binary implements `version`, `help`, `validate`, `canonicalize`, and `serve` (SMTP only; accepted mail is discarded). There is **no queryable inbox**, REST, MCP, auth, UI, or container image yet.

Module [`github.com/hilather/go-lab-maildev`](https://github.com/hilather/go-lab-maildev) · Binary `labmail` · Image (later) `ghcr.io/hilather/labmail` · YAML `apiVersion: labmail.dev/v1alpha1`, `kind: LabMail`

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
| Management / UI / REST | 1080 | inspect captured mail (`/email` compat; native `/v1` + `/mcp` later) |

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
./bin/labmail serve --config testdata/config/valid/defaults.yaml --smtp-listen 127.0.0.1:1025
```

`serve` binds SMTP from the compiled YAML (override with `--smtp-listen`). Accepted messages are discarded to a Null sink. Management HTTP on `:1080` is not bound yet.

## Build and test

```bash
make format
make lint
make test
make test-config-compat
make test-docs
make build
```

Required CI jobs: format, lint, unit, documentation. There is no optional or bypassable job.

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
| [tasks/00-program-board.md](https://github.com/hilather/go-lab-maildev/blob/main/tasks/00-program-board.md) | PRs 1–14 |
| [CHANGELOG.md](https://github.com/hilather/go-lab-maildev/blob/main/CHANGELOG.md) | Curated history |

## License

[Apache License 2.0](https://github.com/hilather/go-lab-maildev/blob/main/LICENSE)
