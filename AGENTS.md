# Agent Implementation Instructions

Status: mandatory
Applies to: every code, configuration, test, schema, UI, deployment, and documentation change
Last updated: 2026-08-17

These instructions apply to every human or AI agent working in this repository. Nested `AGENTS.md` files or `.cursor/rules` may add stricter rules but may not weaken this file.

## 1. Mission

Build **LabMail** (`labmaild`): a Go-native, **receive-only SMTP sink** that is a laboratory-constrained parity rewrite of [MailDev](https://github.com/maildev/maildev). Systems under test send mail here. The sink captures it for inspection through REST, MCP, and an embedded web UI. It **never relays or sends mail outward**.

This appliance belongs with:

- [LabDNS](https://github.com/hilather/go-lab-dns)
- [LabLDAP](https://github.com/hilather/go-lab-ldap-mcp)
- [TacLab](https://github.com/hilather/go-lab-tacacs-mcp)

and is intended to replace the off-the-shelf `maildev/maildev:2.2.1` image used by [mcp-integration-lab](https://github.com/hilather/mcp-integration-lab).

Read before changing code:

1. This file (all of it).
2. [START-HERE.md](START-HERE.md) if you are new to the repo.
3. [docs/README.md](docs/README.md) — catalog.
4. [docs/00-evaluation.md](docs/00-evaluation.md) — what MailDev actually is, and what we keep.
5. [docs/01-architecture.md](docs/01-architecture.md)
6. [docs/05-control-plane-and-parity.md](docs/05-control-plane-and-parity.md)
7. [docs/parity-plan.md](docs/parity-plan.md)
8. Every accepted ADR under [docs/adr/](docs/adr/) that touches the change.

On conflict, [docs/01-architecture.md](docs/01-architecture.md) and accepted ADRs win.

## 2. Non-negotiable rules

### 2.1 Receive-only

The process must not open an outbound SMTP client, must not implement working relay or auto-relay, and must reject MailDev `outgoing-*` / `auto-relay*` configuration fail-closed. Captured messages stay in process memory (or a lab tmpfs directory). Restart and `mail_state_reset` wipe them. Do not write captured mail back into the bootstrap YAML.

### 2.2 REST and MCP share one application layer

REST handlers, MCP tools/resources, the WebSocket notifier, and the UI must call the same `internal/app` operations. Do not implement business logic in adapters. Do not implement MCP by proxying to REST over localhost, or REST by invoking MCP.

Every public REST control capability that an operator or agent uses to inspect or change captured mail, configuration visibility, or sink state is `PARITY_REQUIRED` unless an ADR marks it `REST_ONLY_PROTOCOL` (health, OpenAPI, SPA bootstrap, WebSocket framing) or `MCP_ONLY_PROTOCOL` (prompts, protocol subscriptions).

### 2.3 Always add regression tests

Every behavior change and every defect fix requires automated regression coverage in the same change.

- A bug fix must begin with or include a test that fails before the fix and passes after it.
- New SMTP behavior requires protocol-level tests (greeting, MAIL/RCPT/DATA, AUTH, SIZE, hidden extensions, 8BITMIME/SMTPUTF8 as advertised).
- New MIME/HTML behavior requires fixture tests (multipart, attachments, CID, encoding, sanitizer).
- New REST functionality requires contract tests against OpenAPI.
- New MCP functionality requires protocol tests and REST/MCP parity tests.
- New UI operator workflows require a Playwright (or equivalent) path, not a screenshot-only check.
- Configuration changes require valid, invalid, unknown-field, env/flag overlay, and MailDev-flag compatibility tests.
- Never delete or weaken a test merely to make CI pass unless the test is provably incorrect; document the reason in the change.

Do not mark a task complete using manual testing alone.

### 2.4 Keep documentation up to date in the same change

Documentation is part of the implementation. A change is incomplete until every affected surface is updated:

- architecture, SMTP semantics, mail model, REST, MCP, parity matrix
- configuration schema, examples, CLI help
- security, threat model, operations
- task status / wave board
- `CHANGELOG.md` `[Unreleased]`
- this file, when rules or layout change

Stale documentation is a defect. Do not change an architectural invariant without an ADR.

### 2.5 Releases must describe all changes between versions

Every release tag requires:

1. A complete `## [vX.Y.Z] — YYYY-MM-DD` section in `CHANGELOG.md` that is the operator-facing delta since the previous tag (or since repository start for the first tag). Promote every `[Unreleased]` item. A raw `git log` or PR list is not sufficient.
2. A curated notes file `docs/releases/vX.Y.Z.md` that uses every heading in [RELEASE-NOTES-TEMPLATE.md](RELEASE-NOTES-TEMPLATE.md).
3. Green CI on the **exact** tag commit.

Cover additions, behavior changes, bug fixes, removals, security, REST, MCP, SMTP, UI, configuration/schema, deployment, compatibility, migrations, and known limitations.

### 2.6 After every PR or PR chain, watch CI and harden failures

These rules apply to every agent after a push, PR update, merge, or git tag.

1. Identify the GitHub Actions run for the ref you just changed.
2. Wait for completion. Do not declare the change done while that run is queued, in progress, or red.
3. On failure: read the failed job logs, fix the **root cause**, and **harden** so the same class of failure cannot recur (regression test, fail-closed guard, pin, timeout, diagnostic, or workflow change). Push and watch again.
4. Do not hide flakes with broad retries. A flake is a product or pipeline bug until proven otherwise.
5. Do not bypass, skip, mark optional, or administratively override a failing required check to ship.
6. **PR chains / merge trains.** When two or more PRs will be merged in one session, watch CI on the **last** PR in that series until `ci-gate` is green on the head being merged, then wait for the `main` push `ci-gate` after that last merge. Do not walk away from a red `main` tip. A single PR, a lone push, or a tag still uses items 1–5 in full.
7. A red tag is a release blocker. Move the tag onto a green commit or publish a patch tag. Do not leave a published `v*` pointing at a failing tree.

Record hardening in `docs/ci-failure-hardening/` using [CI-FAILURE-HARDENING-TEMPLATE.md](CI-FAILURE-HARDENING-TEMPLATE.md).

### 2.7 Fail closed

- Unknown configuration fields are errors.
- Invalid YAML, flags, or env overlays leave the process unstarted (startup) or the previous snapshot active (reload, if reload is ever added).
- Missing authentication on the management listener is 401 (except documented unauthenticated probes).
- Missing authorization scope denies the operation.
- Oversized messages are rejected at SMTP before they are stored.
- Relay/outbound configuration is rejected at config compile time.
- Do not log passwords, bearer tokens, SMTP AUTH secrets, or raw Authorization headers.

### 2.8 Generated files

Do not hand-edit generated OpenAPI, JSON Schema, MCP manifests, capability maps, mocks, or generated documentation. Change the source and run the documented generation target. Generation verification must leave the worktree clean.

## 3. Expected layout

```text
.
|-- AGENTS.md
|-- README.md
|-- CHANGELOG.md
|-- Makefile
|-- go.mod
|-- cmd/labmaild/
|-- internal/
|   |-- app/            application operations (the only business logic)
|   |-- model/          canonical mail + config types
|   |-- config/         YAML, flags, env; reject relay
|   |-- store/          ephemeral inbox
|   |-- smtpd/          SMTP listener (receive-only)
|   |-- mime/           parse, CID, attachments
|   |-- sanitize/       HTML sanitizer adapter
|   |-- control/rest/   HTTP adapter
|   |-- control/mcp/    MCP adapter
|   |-- control/ws/     native WebSocket events
|   |-- capabilities/   registry (no app import)
|   |-- auth/           basic + bearer
|   |-- domainerr/
|   `-- web/            embedded SPA
|-- api/                OpenAPI, MCP manifest, schemas (generated)
|-- web/                MailDev 3.0 UI fork (TypeScript/React)
|-- config/             schema + examples
|-- deploy/
|-- test/
|-- docs/
`-- tools/
```

A different layout requires an ADR and must preserve package boundaries.

## 4. Workflow for every task

1. Identify the wave/task ID in [docs/waves/](docs/waves/).
2. Confirm dependencies are complete and you own the exclusive files.
3. Restate acceptance criteria in the PR.
4. Write or update tests first when practical.
5. Implement the smallest change that satisfies the task.
6. Update generated files through committed commands.
7. Update documentation and `CHANGELOG.md` `[Unreleased]`.
8. Run the required local checks.
9. After push, watch CI (§2.6).

If a task cannot be completed without violating an architecture rule, stop and write or amend an ADR rather than working around the rule.

## 5. Definition of done

A task is done only when:

- Production code is complete and contains no placeholder behavior for in-scope features.
- Public contracts are in typed schemas.
- Regression and acceptance tests pass.
- REST/MCP parity checks pass for any control-plane change.
- Secret-redaction tests pass for any auth or logging change.
- Documentation is current.
- `CHANGELOG.md` `[Unreleased]` mentions user-visible changes.
- No TODO remains without a linked task ID.
- CI for the change’s ref is green.

## 6. Required commands

Until the Makefile exists (wave 1), the task that first needs a target must add it. The repository must expose equivalents of:

```text
make format
make lint
make generate
make verify-generated
make test
make test-race
make test-fuzz-smoke
make test-integration
make test-parity
make test-config-compat
make test-docs
make test-container
make security-scan
make test-changelog
make ci
```

`make ci` is the merge-gate equivalent. Do not skip checks to claim completion.

## 7. Coding standards

- Go toolchain: pin in `go.mod` when the module lands; prefer current stable used by sibling appliances unless an ADR says otherwise.
- `context.Context` on all I/O and long-running operations.
- `log/slog` with structured fields and redaction.
- Wrap errors with operation context; use `internal/domainerr` at API boundaries.
- Prefer the standard library and small, well-maintained libraries. Hide SMTP, MIME, MCP, and sanitizer types behind adapters.
- `net/http` with explicit timeouts. Never `panic` for expected input or network errors.
- Constant-time compare for static tokens and SMTP AUTH secrets.
- Frontend: TypeScript strict mode, generated OpenAPI types, no tokens in `localStorage`. Treat server HTML as untrusted; the sanitizer is the enforcement layer for the HTML preview.

## 8. Architecture decision records

Create an ADR for decisions that alter receive-only posture, REST/MCP parity, public configuration schema, authentication, SMTP compatibility, UI vendoring, MCP protocol pin, persistence, or deployment topology.

An ADR must include context, decision, alternatives, consequences, compatibility impact, migration, test impact, and documentation impact.

## 9. Reporting format

At the end of each implementation task, report:

```text
Task: W#-ID
Result: complete | partial | blocked
Files changed: ...
Tests run: ...
Acceptance criteria: pass/fail by item
CI: run URL and conclusion
Docs updated: ...
Release-note entry: ...
Security notes: ...
Follow-up tasks: ...
```

Do not claim completion when required tests or CI were skipped.
