# Start here

LabMail is a receive-only SMTP lab appliance in the LabDNS / LabLDAP / TacLab
family. Systems under test deliver RFC 5321 SMTP to it. LabMail captures,
indexes, and exposes every accepted message over REST, MCP, and an embedded
inbox UI. It never opens an outbound SMTP session and never relays.

- **Run what exists today** — stay on this page, then follow the
  [README](https://github.com/hilather/go-lab-maildev/blob/main/README.md).
- **Change it** — read
  [AGENTS.md](https://github.com/hilather/go-lab-maildev/blob/main/AGENTS.md)
  before touching code.

## Five-minute path

1. Install **Go 1.26** and clone this repository.
2. `go build -o bin/labmail ./cmd/labmail`
3. `./bin/labmail version`
4. `./bin/labmail help`
5. Write a `labmail.dev/v1alpha1` file (or use
   [testdata/config/valid/defaults.yaml](https://github.com/hilather/go-lab-maildev/blob/main/testdata/config/valid/defaults.yaml)).
6. `./bin/labmail validate --config testdata/config/valid/defaults.yaml`
7. `./bin/labmail serve --config testdata/config/valid/defaults.yaml --smtp-listen 127.0.0.1:1025 --management-listen 127.0.0.1:1080`

`validate` and `canonicalize` load one fail-closed YAML document.
`serve` binds SMTP, native `/v1` REST, the inbox SPA at `/`, `POST /mcp`,
and maildev `/email` compat (when `compatEnabled`). Default lab auth is
`bearer_and_basic`. For a local browser session without tokens, set
`spec.management.auth.mode: dev-loopback-unauth`. If hashed SPA JS returns
`403` `origin is not allowed` from a LAN or Codespaces Origin, hatch
`spec.management.originAllowlist` (`"*"`, `"private"`, or exact) —
[origin cookbook](https://github.com/hilather/go-lab-maildev/blob/main/docs/11-deployment.md#origin-allowlist-cookbook).

Read the loaded snapshot with `GET /v1/state`. Validate a candidate with
`POST /v1/state:validate`. Dry-run and commit mutations with
`POST /v1/changes:plan` and `POST /v1/changes:apply`. Reload the bootstrap
file and wipe the inbox with `POST /v1/state:reset`. Curl and MCP examples
are in the [README state loading section](https://github.com/hilather/go-lab-maildev/blob/main/README.md#state-loading-apis).

`healthcheck` probes `GET /v1/health/ready`. `mcp-stdio` is the developer
stdio adapter (`--token-file` is verified). Session cookie `labmail_session`
+ `X-LabMail-CSRF` is REST-only. `make web-build` (Node **22.14.0**) embeds
the production SPA. The hardened image and compose smoke are in
[docs/11-deployment.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/11-deployment.md);
`make test-container` needs Docker.

YAML field rules, revisions, and reset:
[docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md).
SMTP accept/reject tables:
[docs/02-smtp-semantics.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/02-smtp-semantics.md).
REST and MCP twins:
[docs/06-rest-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/06-rest-api.md)
and
[docs/07-mcp-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/07-mcp-api.md).

## What to read next

| If you are… | Read |
|---|---|
| Running a lab | [README.md](https://github.com/hilather/go-lab-maildev/blob/main/README.md), [docs/11-deployment.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/11-deployment.md), [docs/13-integration-lab-swap.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/13-integration-lab-swap.md) |
| Writing YAML or calling the state APIs | [docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md), [README state loading](https://github.com/hilather/go-lab-maildev/blob/main/README.md#state-loading-apis) |
| Implementing SMTP | [docs/02-smtp-semantics.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/02-smtp-semantics.md), [docs/adr/0002-in-tree-smtp-receive-only.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0002-in-tree-smtp-receive-only.md) |
| Wiring an agent | [docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md), [docs/07-mcp-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/07-mcp-api.md) |
| Keeping maildev smoke green | [docs/12-maildev-compat.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/12-maildev-compat.md) |
| Changing behavior | [AGENTS.md](https://github.com/hilather/go-lab-maildev/blob/main/AGENTS.md), then the design doc for that area |

The full catalog — architecture, ADRs, task lists — is in
[docs/README.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/README.md)
and linked from the
[README documentation map](https://github.com/hilather/go-lab-maildev/blob/main/README.md#documentation).

## For contributors and agents

Before changing code:

1. Read [AGENTS.md](https://github.com/hilather/go-lab-maildev/blob/main/AGENTS.md) completely.
2. Read architecture, SMTP semantics, store, state, control-plane parity, security, and testing:
   [docs/01-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/01-architecture.md),
   [docs/02-smtp-semantics.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/02-smtp-semantics.md),
   [docs/03-message-store.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/03-message-store.md),
   [docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md),
   [docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md),
   [docs/08-security-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/08-security-architecture.md),
   [docs/10-testing-strategy.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/10-testing-strategy.md).
3. Read every ADR that affects the task (`docs/adr/`).
4. Take one work package from
   [tasks/00-program-board.md](https://github.com/hilather/go-lab-maildev/blob/main/tasks/00-program-board.md)
   whose dependencies are complete.
5. Add or update tests before declaring the task done.
6. Update every affected document in the same change.
7. Run every required local verification target (`make test`, `make test-docs`,
   and the rest listed in
   [AGENTS.md](https://github.com/hilather/go-lab-maildev/blob/main/AGENTS.md)).

Do not implement REST, MCP, SMTP, configuration, or the store from a task
summary when a design document exists. The numbered pack is the source of
truth. If an invariant must change, write an ADR and update the design
documentation first.

Coordinators allocate work with
[tasks/00-program-board.md](https://github.com/hilather/go-lab-maildev/blob/main/tasks/00-program-board.md).
Parallel work is safe only when package ownership and schema ownership do not
overlap. Integration changes to shared domain types, generated schemas, or the
capability registry must be serialized.

### Definition of done

A task is not done until:

- The implementation is complete and reviewable.
- Unit and regression tests cover all changed behavior.
- Protocol changes have integration and compatibility tests.
- Configuration changes have positive and negative validation tests.
- REST and MCP parity checks pass (`make test-parity`).
- Race, fuzz, lint, generation, documentation, and relevant end-to-end checks pass.
- All affected documentation is current.
- No CI check is ignored, bypassed, or marked optional to get a merge.
- User-visible and operator-visible changes are recorded in
  [CHANGELOG.md](https://github.com/hilather/go-lab-maildev/blob/main/CHANGELOG.md).

1.0 includes the embedded inbox UI. The swap-release labinfo id stays
`maildev`. LabMail is a lab sink, not a public MTA. Residuals:
[docs/known-limitations.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/known-limitations.md).
Current notes:
[docs/releases/v1.0.0-rc.3.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.3.md).
