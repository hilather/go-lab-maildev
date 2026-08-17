# Acceptance criteria (first GA)

Status: Proposed
Last reviewed: 2026-08-17

A GA candidate (`v1.0.0` or `v1.0.0-rc.1`) requires all of the following, with tests or recorded evidence.

## Ingest

- [ ] Anonymous SMTP accepts a plain message from Go `net/smtp`.
- [ ] AUTH PLAIN works when configured and rejects bad passwords.
- [ ] Oversized DATA is rejected and not stored.
- [ ] Inbox full returns 452 and does not evict silently.
- [ ] UTF-8 subject/body stored correctly when SMTPUTF8 advertised.
- [ ] `--hide-extensions STARTTLS` removes STARTTLS from EHLO.

## Inspection

- [ ] Dual prefix: `/email` and `/api/email` list/get/delete/html/source/download/attachment.
- [ ] GET by id marks read on REST **and** MCP.
- [ ] Dotted filters and skip pagination match documented semantics.
- [ ] HTML sanitizer strips `script`; CID images resolve.
- [ ] Config JSON has `isOutgoingEnabled: false`.

## Control plane

- [ ] Capability registry has no `PARITY_REQUIRED` holes.
- [ ] `make test-parity` passes.
- [ ] MCP `tools/list` + search/get/delete/delete_all/html/source/attachment/reset/wait.
- [ ] Bearer and basic both work on REST and MCP.
- [ ] Relay flags refuse process start.

## UI

- [ ] Embedded SPA lists mail and live-updates on `/ws`.
- [ ] No Relay control visible.
- [ ] Playwright (or equivalent) paths green.

## Lab

- [ ] Container: non-root, read-only, cap_drop, healthcheck binary.
- [ ] Documented compose + MAILDEV_ARGS mode.
- [ ] Hypothetical lab smoke REST assertions would pass (tested in this repo’s compose smoke).

## Process

- [ ] `CHANGELOG.md` and `docs/releases/<tag>.md` complete.
- [ ] Required CI green on the tag SHA.
- [ ] `govulncheck` clean of high/critical in our code and direct deps (or documented waiver).
- [ ] AGENTS.md rules still accurate.

Evidence index will live at `docs/releases/acceptance-evidence.md` when GA work starts.
