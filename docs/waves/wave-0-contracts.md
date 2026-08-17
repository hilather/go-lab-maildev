# Wave 0 — Contracts

Status: in-progress (this documentation PR)
Dependencies: none
Exclusive ownership: `docs/**`, `AGENTS.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `.cursor/rules/**`, templates, `README.md`

## Goal

Evaluate MailDev and mcp-integration-lab, freeze architecture and parity, and give later agents parallelizable tasks plus mandatory rules for tests, docs, release notes, and CI hardening.

## Tasks

### W0-DOC — Design pack

Status: in-progress

- [x] Evaluate MailDev 2.2.1 (lab) and 3.0 (upstream main)
- [x] Map lab compose/smoke/labinfo/maildev.yaml constraints
- [x] Architecture, SMTP, model, config, REST, MCP, UI, security, testing, deployment, lab cutover
- [x] ADRs 0001–0012
- [x] Parity plan with three axes
- [x] Wave board and parallelization
- [x] AGENTS.md + cursor rules (regression tests, docs, release notes, CI watch/hardening)
- [x] CHANGELOG `[Unreleased]` for this pack

Acceptance:

- A new agent can implement wave 1 without inventing capability names or relay behavior.
- Receive-only and REST/MCP parity are unambiguous.

## Explicit non-scope

Go module, SMTP server, UI vendor copy (wave 3), mcp-integration-lab code changes.

## Handoff

Merge this PR. Mark wave 0 **done** on the program board in a tiny follow-up if the merge commit should show `done` rather than `in-progress`.
