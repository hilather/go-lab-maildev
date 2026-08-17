# Documentation

Operator front door: [README.md](https://github.com/hilather/go-lab-maildev/blob/main/README.md). Onboarding: [START-HERE.md](https://github.com/hilather/go-lab-maildev/blob/main/START-HERE.md). Agent rules: [AGENTS.md](https://github.com/hilather/go-lab-maildev/blob/main/AGENTS.md).

This page is the catalog. Normative design documents win over task summaries. After FND-001 the numbered pack is the source of truth.

## Root

| Path | Role |
|---|---|
| [README.md](https://github.com/hilather/go-lab-maildev/blob/main/README.md) | Product page |
| [START-HERE.md](https://github.com/hilather/go-lab-maildev/blob/main/START-HERE.md) | Onboarding and definition of done |
| [AGENTS.md](https://github.com/hilather/go-lab-maildev/blob/main/AGENTS.md) | Mandatory contributor / agent instructions |
| [CHANGELOG.md](https://github.com/hilather/go-lab-maildev/blob/main/CHANGELOG.md) | Curated history |
| [LICENSE](https://github.com/hilather/go-lab-maildev/blob/main/LICENSE) | Apache-2.0 |

## Architecture

| Path | Topic |
|---|---|
| [01-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/01-architecture.md) | Process and package model |
| [02-smtp-semantics.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/02-smtp-semantics.md) | Command table, limits, AUTH/TLS |
| [03-message-store.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/03-message-store.md) | Caps, wait, wipe, spill |
| [04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md) | YAML, revisions, reset |
| [05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md) | Shared capability registry |

## Interfaces

| Path | Topic |
|---|---|
| [06-rest-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/06-rest-api.md) | REST `/v1` |
| [07-mcp-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/07-mcp-api.md) | MCP tools and protocol pin |
| [09-observability.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/09-observability.md) | Metrics, logs, health |
| [12-maildev-compat.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/12-maildev-compat.md) | maildev `/email` shim |

## Security, operations, release

| Path | Topic |
|---|---|
| [08-security-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/08-security-architecture.md) | Authn/z, XSS, receive-only |
| [10-testing-strategy.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/10-testing-strategy.md) | Test layers |
| [11-deployment.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/11-deployment.md) | Container and process |
| [13-integration-lab-swap.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/13-integration-lab-swap.md) | mcp-integration-lab swap |

## Architecture decisions

| ADR | Decision |
|---|---|
| [0001](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0001-use-go.md) | Use Go |
| [0002](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0002-in-tree-smtp-receive-only.md) | In-tree SMTP, receive-only (D7/D8) |
| [0003](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0003-ephemeral-inbox-and-gitops.md) | Ephemeral inbox and GitOps (D3) |
| [0004](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0004-shared-capability-registry.md) | Shared capability registry (D4) |
| [0005](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0005-lab-static-bearer-and-basic-compat.md) | Lab static bearer + Basic compat (D6) |
| [0006](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0006-pin-mcp-protocol-versions.md) | Pin MCP protocol versions (D14 + D17) |
| [0007](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0007-compat-email-surface.md) | Compat `/email` surface (D5) |

## Task lists

| Path | Package |
|---|---|
| [00-program-board.md](https://github.com/hilather/go-lab-maildev/blob/main/tasks/00-program-board.md) | PRs 1–14 and milestones |
| [README.md](https://github.com/hilather/go-lab-maildev/blob/main/tasks/README.md) | Task working rules |
