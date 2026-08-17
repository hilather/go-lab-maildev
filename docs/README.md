# Documentation

Operator front door: [README.md](../README.md). Agent onboarding: [START-HERE.md](../START-HERE.md). Agent rules: [AGENTS.md](../AGENTS.md). Contributing: [CONTRIBUTING.md](../CONTRIBUTING.md).

This catalog is the index. Normative design and accepted ADRs win over wave summaries.

Status: **design pack for the Go rewrite**. There is no `labmaild` implementation yet. Wave 0 is this documentation.

## How to use this pack

1. Read [00-evaluation.md](00-evaluation.md) to see what MailDev 2.2.1 (lab image) and MailDev 3.0 (upstream main) actually provide.
2. Read [01-architecture.md](01-architecture.md) and [parity-plan.md](parity-plan.md) for the LabMail target.
3. Pick a wave from [waves/00-program-board.md](waves/00-program-board.md). Parallel lanes are in [waves/parallelization-plan.md](waves/parallelization-plan.md).
4. Implement against exclusive file ownership in the wave file. Do not weaken [AGENTS.md](../AGENTS.md).

## Root

| Path | Role |
| --- | --- |
| [README.md](../README.md) | Product page |
| [START-HERE.md](../START-HERE.md) | Agent onboarding |
| [AGENTS.md](../AGENTS.md) | Mandatory contributor / agent instructions |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | PR workflow |
| [CHANGELOG.md](../CHANGELOG.md) | Curated history |
| [LICENSE](../LICENSE) | Apache-2.0 |
| [RELEASE-NOTES-TEMPLATE.md](../RELEASE-NOTES-TEMPLATE.md) | Between-tag notes template |
| [CI-FAILURE-HARDENING-TEMPLATE.md](../CI-FAILURE-HARDENING-TEMPLATE.md) | CI hardening record |

## Architecture and evaluation

| Path | Topic |
| --- | --- |
| [00-evaluation.md](00-evaluation.md) | MailDev 2.2.1 / 3.0 evaluation |
| [01-architecture.md](01-architecture.md) | System model, packages, flows |
| [implementation-design.md](implementation-design.md) | Implementation design |
| [parity-plan.md](parity-plan.md) | MailDev + lab + REST/MCP parity |
| [02-smtp-semantics.md](02-smtp-semantics.md) | SMTP ingest contract |
| [03-mail-model.md](03-mail-model.md) | Captured-mail domain types |
| [04-state-and-configuration.md](04-state-and-configuration.md) | YAML, flags, env, ephemeral store |

## Interfaces

| Path | Topic |
| --- | --- |
| [05-control-plane-and-parity.md](05-control-plane-and-parity.md) | Capability registry |
| [06-rest-api.md](06-rest-api.md) | REST (v2 `/email` + v3 `/api`) |
| [07-mcp-api.md](07-mcp-api.md) | MCP tools, resources, protocol pin |
| [13-frontend.md](13-frontend.md) | Vendored MailDev 3 UI |
| [17-error-model.md](17-error-model.md) | Domain errors |
| [09-observability.md](09-observability.md) | Metrics, logs, health |

## Security, operations, release

| Path | Topic |
| --- | --- |
| [08-security-architecture.md](08-security-architecture.md) | Authn/z, sanitizer, trust boundaries |
| [20-threat-model.md](20-threat-model.md) | Threat model |
| [10-testing-strategy.md](10-testing-strategy.md) | Test layers |
| [11-deployment.md](11-deployment.md) | Container and process |
| [12-lab-integration.md](12-lab-integration.md) | mcp-integration-lab cutover |
| [14-release-engineering.md](14-release-engineering.md) | Tags, notes, CI watch |
| [15-documentation-governance.md](15-documentation-governance.md) | Docs policy |
| [16-compatibility-and-versioning.md](16-compatibility-and-versioning.md) | Compatibility |
| [18-roadmap-and-non-goals.md](18-roadmap-and-non-goals.md) | Roadmap |
| [19-acceptance-criteria.md](19-acceptance-criteria.md) | First-GA acceptance |
| [21-standards-and-references.md](21-standards-and-references.md) | RFCs and upstream |
| [known-limitations.md](known-limitations.md) | First-GA residual |

## Architecture decisions

| ADR | Decision |
| --- | --- |
| [0001](adr/0001-use-go.md) | Use Go |
| [0002](adr/0002-receive-only-sink.md) | Receive-only; no outbound SMTP |
| [0003](adr/0003-shared-capability-registry.md) | Shared capability registry |
| [0004](adr/0004-dual-rest-prefix.md) | Dual REST prefix (`/email` and `/api/email`) |
| [0005](adr/0005-vendor-maildev-3-ui.md) | Vendor MailDev 3.0 React UI |
| [0006](adr/0006-pin-mcp-protocol.md) | Pin MCP 2026-07-28 + legacy-client knob |
| [0007](adr/0007-basic-and-bearer-auth.md) | HTTP basic + bearer |
| [0008](adr/0008-ephemeral-captured-mail.md) | Ephemeral inbox |
| [0009](adr/0009-native-go-smtp.md) | Native Go SMTP + MIME adapters |
| [0010](adr/0010-yaml-plus-maildev-flags.md) | YAML primary, MailDev flags overlay |
| [0011](adr/0011-omit-outbound-relay.md) | Omit relay from REST, MCP, and UI |
| [0012](adr/0012-native-websocket-not-socketio.md) | Native WebSocket instead of Socket.IO |

## Waves (agent work)

| Path | Role |
| --- | --- |
| [waves/00-program-board.md](waves/00-program-board.md) | Milestones and status |
| [waves/parallelization-plan.md](waves/parallelization-plan.md) | Parallel lanes |
| [waves/agent-task-template.md](waves/agent-task-template.md) | Task file template |
| [waves/wave-0-contracts.md](waves/wave-0-contracts.md) | This pack (done when merged) |
| [waves/wave-1-foundation.md](waves/wave-1-foundation.md) | Module, CI, schema, registry |
| [waves/wave-2-ingest.md](waves/wave-2-ingest.md) | SMTP, MIME, store |
| [waves/wave-3-control-plane.md](waves/wave-3-control-plane.md) | App, REST, MCP, WS, UI |
| [waves/wave-4-productize.md](waves/wave-4-productize.md) | Image, lab cutover, parity suite |
| [waves/wave-5-ga.md](waves/wave-5-ga.md) | Interop, release, hardening |

## Releases and hardening

| Path | Role |
| --- | --- |
| [ci-failure-hardening/](ci-failure-hardening/) | Per-incident hardening records (created when CI fails) |
| [releases/](releases/) | Per-tag curated notes (created at first release) |
