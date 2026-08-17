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
| Config compat | flags/env/YAML | relay reject, lab MAILDEV_ARGS |
| Docs | link check, example YAML validate | |
| Changelog | Unreleased format / required headings on tags | |

## Regression rule

Every bug: test first. Every MailDev behavioral quirk we keep (mark-read on GET, JSON `true` deletes, skip pagination) gets a named test so we do not “clean it up” accidentally.

## Fixtures

`testdata/eml/`: plain, html+cid, utf-8, 8bit, attachments, malformed, huge header. Capture a real 2.2.1 JSON once for golden field names.

## What not to do

- Sleep-based flakes without a deadline poll helper.
- Hitting the public internet.
- Skipping `-race` to hide a store bug.
- Mocking SMTP by not speaking SMTP for ingest tests.
