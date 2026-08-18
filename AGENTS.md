# Repository Instructions for Agents

These instructions apply to every human or AI agent working in this repository. More specific `AGENTS.md` files may add stricter rules but may not weaken this file.

## Required reading

Before modifying code, read:

1. `docs/01-architecture.md`
2. `docs/02-smtp-semantics.md`
3. `docs/03-message-store.md`
4. `docs/04-state-and-configuration.md`
5. `docs/05-control-plane-and-parity.md`
6. `docs/08-security-architecture.md`
7. `docs/10-testing-strategy.md`
8. Every ADR relevant to the area being changed

The numbered pack is the source of truth after FND-001. Do not invent paths, types, regexes, validate rules, or capability IDs. If an invariant must change, write an ADR first.

## Architectural rules

- REST and MCP are adapters. Domain behavior belongs in `internal/app`, `internal/store`, `internal/smtp`, `internal/config`, or `internal/model`.
- REST handlers, MCP handlers, and the maildev `/email` compat adapter must never implement independent business logic and must never call each other.
- Every public capability must be represented in the central capability registry.
- SMTP must keep accepting if REST/MCP is slow or unbound. `internal/smtp` must not import `internal/control`, `internal/web`, or `net/http`.
- Receive-only is structural. Production `internal/smtp`, `internal/store`, and `internal/app` must not import `net/smtp` and must not call `net.Dial`, `net.DialTimeout`, or `net.Dialer.Dial`. Listen/Accept only.
- The config loader rejects any key matching `outgoing*`, `auto-relay*`, `relay*`, `smarthost*` (normalized: strip dashes, underscores, and case).
- Compat `POST /email/{id}/relay` always returns 403 `receive_only`. Never implement relay.
- Desired state is YAML. The inbox is not. Reset rereads bootstrap **and** wipes mail.
- The service must not write to the bootstrap configuration file.
- Do not add a database, mail-directory, hidden volume, or other persistence mechanism without an approved ADR.
- Do not import `emersion/go-smtp` in the server (ADR 0002). MIME parsing is isolated in `internal/mimeparse`.
- Do not add implicit SMTPS (`smtp.tls.mode: implicit`) in 1.0.
- Do not add a chaos / fault-injection engine in 1.0.
- The embedded inbox UI is required for GA / 1.0 (PR 12). Do not ship 1.0 without it.
- During the mcp-integration-lab swap release, labinfo catalog id stays `maildev` (D15). Rename only in a later lab release.
- Hide third-party SMTP, MIME, MCP, and YAML library types behind internal adapters.

## Tests and regressions

- Every area must have regression tests.
- Every code path, protocol behavior, API capability, configuration semantic, operational script, and bug fix must have appropriate automated regression coverage.
- A bug fix must begin with or include a test that fails before the fix and passes after it.
- New SMTP behavior requires session transcripts over `internal/smtptest`.
- New REST functionality requires contract tests and shared-domain tests.
- New MCP functionality requires protocol tests and REST/MCP parity tests.
- Configuration changes require valid, invalid, reserved-key, normalization, and revision tests.
- Never delete or weaken a test merely to make CI pass unless the test is provably incorrect; document the reason in the change.

## CI is mandatory

- All required CI checks must pass before merge and before a release tag is created.
- Do not bypass, skip, mark optional, or administratively override a failing check to ship a change.
- Treat every CI failure as either a product defect or a pipeline defect.
- When CI fails, fix the immediate cause and harden the system so that the same failure is easier to diagnose and less likely to recur.
- Do not hide flaky tests with broad retries. Find and remove the source of nondeterminism.
- A task is incomplete until all relevant local and CI-equivalent targets pass.

## Release tags and release notes

- Every release tag must include complete release notes describing all functionality differences from the previous release (`docs/releases/<tag>.md`).
- Release notes must cover additions, behavior changes, bug fixes, removals, security changes, REST changes, MCP changes, SMTP semantics, configuration/schema changes, deployment changes, compatibility impact, migrations, and known limitations.
- Residual limitations live in `docs/known-limitations.md`. Do not claim public-MTA completeness. Do not claim AUTH/STARTTLS, the hardened image, or swap overlay examples unless those siblings are in the tree.
- A `v*` tag is created only after Release `tag-gate` sees every required CI job green on that SHA.
- A raw commit list or automatically generated pull-request list is not sufficient.
- Breaking changes require explicit migration guidance and the version increment required by the compatibility policy.
- Release notes and changelog entries are part of the release artifact and must be reviewed before tagging.

## Documentation is mandatory

- All documentation must be kept up to date.
- Update affected architecture, API, MCP, configuration, security, operation, testing, deployment, task, and ADR documents in the same change as the implementation.
- Stale documentation is a defect and blocks task completion.
- Examples must be tested or generated where practical.
- Internal links, code samples, configuration examples, and command lines must pass documentation checks.
- Update `Last reviewed` metadata when a document receives a substantive review.
- Do not change an architectural invariant without an ADR.
- Cross-file links in README and `docs/` use absolute HTTPS URLs (`https://github.com/hilather/go-lab-maildev/blob/main/...`).

## REST and MCP parity

- Every public REST control capability must have an MCP equivalent except rows marked `REST_ONLY_PROTOCOL`.
- Every state-changing MCP tool must have a REST equivalent.
- Parameterized MCP read tools must have REST equivalents; MCP resources may mirror REST GET representations.
- Both adapters must use the same input and output domain types and the same authorization decision.
- Every mutation must support validation, dry-run planning, optimistic concurrency, idempotency, actor identity, reason, deterministic errors, audit emission, and an atomic commit.
- Run parity verification whenever REST, MCP, schemas, authorization, or application commands change.

## Receive-only correctness

- No SMTP client in production packages.
- No config type for an outgoing host.
- `VRFY` always replies `252` and never implies mailbox existence.
- Successful DATA never opens a TCP connection to deliver.
- Import-boundary tests fail on any `Dial` ident in `internal/smtp`, `internal/store`, or `internal/app`.

## Generated files

- Do not manually edit generated OpenAPI, JSON Schema, MCP manifest, mocks, golden capability maps, or generated documentation.
- Change the source model or specification and run the documented generation target.
- Generation verification must leave the worktree clean.

## Dependencies

- Prefer the Go standard library and small, well-maintained libraries.
- Pin direct dependencies and review transitive changes.
- Allowed 1.0 direct deps: `gopkg.in/yaml.v3`, `github.com/modelcontextprotocol/go-sdk v1.7.0`, `github.com/emersion/go-message` (MIME adapter only), `github.com/oklog/ulid/v2`.
- No Prometheus client (`github.com/prometheus/*` forbidden). Metrics are hand-rolled OpenMetrics.
- No SMTP/MIME/HTTP frameworks beyond the allowlist. New deps need a PR justification and license check (Apache-2.0 compatible).

## Required completion commands

The implementation repository should provide equivalent targets for:

```text
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
make security-scan
make test-changelog
make web-test
make web-build
```

If a target does not yet exist, the task that first needs it must add it rather than silently omitting the check. Placeholders must fail closed, not succeed as no-ops.

FND-001 required CI jobs: `format`, `lint`, `unit`, `documentation`. API-001 adds `generated-file`. MCP-001 adds `parity`. DEP-001 adds `container-test`. UI-001 adds `web`.
