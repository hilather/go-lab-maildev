# Program board

Status: wave 0 in progress (documentation pack)
Last reviewed: 2026-08-17

Implementation has **not** started. Agents pick tasks from wave files. Do not start a later wave until the board marks its dependencies ready, except where the parallelization plan allows interface-only work.

## Waves

| Wave | File | Goal | Parallelism | Status |
| ---: | --- | --- | --- | --- |
| 0 | [wave-0-contracts.md](wave-0-contracts.md) | Evaluation, architecture, ADRs, parity, agent rules | Single pack | **this PR** |
| 1 | [wave-1-foundation.md](wave-1-foundation.md) | Module, Make/CI, schema, errors, empty registry | Mostly serial, then 4 lanes | not-started |
| 2 | [wave-2-ingest.md](wave-2-ingest.md) | SMTP, MIME, sanitizer, store | **4 parallel lanes** after W1-CFG interfaces | not-started |
| 3 | [wave-3-control-plane.md](wave-3-control-plane.md) | App, REST, MCP, WS, UI | App first, then **4 parallel adapters** | not-started |
| 4 | [wave-4-productize.md](wave-4-productize.md) | Image, compose, parity suite, lab cutover docs | **4 parallel lanes** | not-started |
| 5 | [wave-5-ga.md](wave-5-ga.md) | Interop, release automation, hardening | Mix; GA integrator serial | not-started |

## Milestones

### M0 — Contracts exist

Wave 0 merged. Agents have frozen capability names, receive-only rule, dual REST prefix, MCP pin.

### M1 — Module compiles and CI is fail-closed

Wave 1: `go test ./...` on stubs, schema validates examples, registry test fails if a PARITY_REQUIRED row lacks bindings (bindings may still be empty stubs that panic until wave 3 — prefer compile-time interfaces).

### M2 — Mail can be captured without HTTP

Wave 2: send SMTP to `127.0.0.1:0`, message in store, parse/sanitize tests green.

### M3 — Agents and browsers can inspect

Wave 3: REST dual prefix, MCP tools, UI list/get/delete, WS live update, parity tests.

### M4 — Lab-shaped image

Wave 4: distroless image, compose, healthcheck, MAILDEV_ARGS mode, documented lab PR recipe.

### M5 — GA candidate

Wave 5: interop clients, changelog/release-diff, acceptance evidence, green tag-gate.

## Cross-cutting freeze (do not bikeshed in later waves)

Owned by wave 0/1. Changing these requires an ADR and a board update:

- Product/binary/MCP names
- Capability IDs and `mail_*` tool names
- Dual REST prefixes
- Receive-only
- MCP 2026-07-28 + legacy knob
- Email JSON field names
- Inbox-full = 452 (no silent eviction)
- GET-by-id marks read

## How to assign parallel agents

1. Read [parallelization-plan.md](parallelization-plan.md).
2. Take a task whose dependencies are `done` and whose exclusive paths do not overlap an in-flight PR.
3. Open one PR per task unless the wave says a slice is a single PR.
4. After push, watch CI ([AGENTS.md](../../AGENTS.md) §2.6).
5. Mark the task done on this board in the same PR as the implementation (edit the wave file).
