# Documentation governance

Status: Proposed normative
Last reviewed: 2026-08-17

## Rule

A behavior change is incomplete until affected docs in this tree are updated in the **same change**. Stale docs are defects.

## Surfaces

| Surface | When to update |
| --- | --- |
| Architecture / SMTP / model / REST / MCP / parity | Contract change |
| ADRs | Invariant change |
| Wave files | Task status, new exclusive files |
| AGENTS.md / .cursor/rules | Process change |
| README | Operator-facing ports, commands |
| CHANGELOG `[Unreleased]` | User-visible |
| known-limitations | Residual behavior |
| NOTICE | Vendored UI / extra licenses |

## Generated docs

Never hand-edit generated OpenAPI/MCP/schema once generation exists. Wave 1 adds `make verify-generated`.

## Review metadata

Keep `Last reviewed: YYYY-MM-DD` accurate on normative docs when they change.

## Examples

YAML examples under `config/examples/` must validate against schema once schema exists (`make test-docs`).
