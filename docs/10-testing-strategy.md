# Testing strategy

Status: Proposed normative
Last reviewed: 2026-08-17
Related: [AGENTS.md](../AGENTS.md) §2.3

## Layers

| Layer | Where | Must cover |
| --- | --- | --- |
| Unit | `internal/*` | parse, filter, sanitizer, config merge, domainerr, capabilities |
| Protocol | `internal/smtpd`, `internal/mime` | SMTP dialogs, EML fixtures |
| Contract | `internal/control/rest` | OpenAPI, MailDev paths, 401 |
| Protocol MCP | `internal/control/mcp` | tools/list, call, resources, prompts |
| Parity | `test/parity` | REST vs MCP same app |
| Race | `go test -race ./...` | store + smtp + wait |
| Fuzz smoke | mime parser, dotted filter | `-fuzztime=0` on CI |
| UI | `web/` Playwright | workflows in [13-frontend.md](13-frontend.md) |
| Container | compose | SMTP + `/email` + `/mcp` |
| Config compat | flags/env/YAML | MailDev flags including outgoing; lab MAILDEV_ARGS without relay |
| Comparison lab | `test/parity-lab` | Every [23-behavior-parity-matrix.md](23-behavior-parity-matrix.md) REST + UI row vs live MailDev 2.2.1 and 3.0; see [22-comparison-lab.md](22-comparison-lab.md) |
| Docs | link check, example YAML validate | |
| Changelog | Unreleased format / required headings on tags | |

## Regression rule

Every bug: test first. Every MailDev behavioral quirk we keep (mark-read on GET, JSON `true` deletes, skip pagination) gets a named test so we do not “clean it up” accidentally.

## Fixtures

`testdata/eml/`: plain, html+cid, utf-8, 8bit, attachments, malformed, huge header. Capture a real 2.2.1 JSON once for golden field names.

## Comparison lab (required for MailDev-facing work)

A MailDev process feature is not done until the matching matrix row passes against **original MailDev** and **LabMail** through REST (and UI when the UI exposes it). Oracle-only CI may land first. After the LabMail image exists, skipping LabMail rows is forbidden.

Relay tests use compose service `relay-sink` only. Do not point `outgoing-host` at the public internet.

## What not to do

- Sleep-based flakes without a deadline poll helper.
- Hitting the public internet (including any public MTA).
- Skipping `-race` to hide a store bug.
- Mocking SMTP by not speaking SMTP for ingest tests.
- Declaring UI parity from screenshots alone; drive workflows with Playwright.
