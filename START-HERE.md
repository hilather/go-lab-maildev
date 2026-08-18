# Start here

LabMail is a receive-only SMTP lab appliance in the LabDNS / LabLDAP / TacLab family. Systems under test deliver RFC 5321 SMTP to it. LabMail captures, indexes, and (once implemented) exposes every accepted message over REST, MCP, and an embedded inbox UI. It never opens an outbound SMTP session and never relays.

If you want to run what exists today, stay on this page, then follow the [README](README.md). If you want to change it, read [AGENTS.md](AGENTS.md) before touching code.

## Five-minute path

1. Install **Go 1.26** and clone this repository.
2. `go build -o bin/labmail ./cmd/labmail`
3. `./bin/labmail version`
4. `./bin/labmail help`
5. `./bin/labmail serve --config testdata/config/valid/defaults.yaml --smtp-listen 127.0.0.1:1025`

`validate` and `canonicalize` load a fail-closed `labmail.dev/v1alpha1` document. `serve` binds SMTP and stores accepted mail in the process inbox. `healthcheck` and management HTTP are not implemented.

YAML field rules, revisions, and reset live in [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md). SMTP accept/reject tables live in [docs/02-smtp-semantics.md](docs/02-smtp-semantics.md). REST and MCP twins are in [docs/06-rest-api.md](docs/06-rest-api.md) and [docs/07-mcp-api.md](docs/07-mcp-api.md).

## What to read next

| If you are… | Read |
|---|---|
| Running a lab (later) | [README.md](README.md), [docs/11-deployment.md](docs/11-deployment.md), [docs/13-integration-lab-swap.md](docs/13-integration-lab-swap.md) |
| Writing YAML | [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md) |
| Implementing SMTP | [docs/02-smtp-semantics.md](docs/02-smtp-semantics.md), [docs/adr/0002-in-tree-smtp-receive-only.md](docs/adr/0002-in-tree-smtp-receive-only.md) |
| Wiring an agent | [docs/05-control-plane-and-parity.md](docs/05-control-plane-and-parity.md), [docs/07-mcp-api.md](docs/07-mcp-api.md) |
| Keeping maildev smoke green | [docs/12-maildev-compat.md](docs/12-maildev-compat.md) |
| Changing behavior | [AGENTS.md](AGENTS.md), then the normative doc for that area |

The full catalog — architecture, ADRs, task lists — is in [docs/README.md](docs/README.md) and linked from the [README documentation map](README.md#documentation).

## For contributors and agents

Before changing code:

1. Read [AGENTS.md](AGENTS.md) completely.
2. Read architecture, SMTP semantics, store, state, control-plane parity, security, and testing: [docs/01-architecture.md](docs/01-architecture.md), [docs/02-smtp-semantics.md](docs/02-smtp-semantics.md), [docs/03-message-store.md](docs/03-message-store.md), [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md), [docs/05-control-plane-and-parity.md](docs/05-control-plane-and-parity.md), [docs/08-security-architecture.md](docs/08-security-architecture.md), [docs/10-testing-strategy.md](docs/10-testing-strategy.md).
3. Read every ADR that affects the task (`docs/adr/`).
4. Take one work package from [tasks/00-program-board.md](tasks/00-program-board.md) whose dependencies are complete.
5. Add or update tests before declaring the task done.
6. Update every affected document in the same change.
7. Run every required local verification target (`make test`, `make test-docs`, and the rest listed in [AGENTS.md](AGENTS.md)).

Do not implement REST, MCP, SMTP, configuration, or the store from a task summary when a normative design document exists. The numbered pack is the source of truth. If an invariant must change, write an ADR and update the normative documentation first.

Coordinators allocate work with [tasks/00-program-board.md](tasks/00-program-board.md). Parallel work is safe only when package ownership and schema ownership do not overlap. Integration changes to shared domain types, generated schemas, or the capability registry must be serialized.

### Definition of done

A task is not done until:

- The implementation is complete and reviewable.
- Unit and regression tests cover all changed behavior.
- Protocol changes have integration and compatibility tests.
- Configuration changes have positive and negative validation tests.
- REST and MCP parity checks pass (once those targets exist).
- Race, fuzz, lint, generation, documentation, and relevant end-to-end checks pass.
- All affected documentation is current.
- No CI check is ignored, bypassed, or marked optional to get a merge.
- User-visible and operator-visible changes are recorded in [CHANGELOG.md](CHANGELOG.md).

GA / 1.0 is not done without the embedded inbox UI (PR 12). The swap-release labinfo id stays `maildev`.
