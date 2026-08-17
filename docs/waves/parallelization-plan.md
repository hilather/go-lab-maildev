# Parallelization plan

Status: Proposed
Last reviewed: 2026-08-17

## Principle

Parallelize around **stable interfaces and exclusive file ownership**, not around shared god files. High-conflict surfaces need one owner:

| Surface | Owner task |
| --- | --- |
| `internal/model` types | W1-MODEL |
| YAML schema + flag table | W1-CFG |
| `internal/domainerr` | W1-ERR |
| `internal/capabilities` IDs | W1-CAP |
| `internal/app` method set | W3-APP |
| OpenAPI merge | W3-REST (MCP contributes fixtures, does not rewrite paths) |
| `web/` fork | W3-UI |
| Dockerfile | W4-DEP |
| `deploy/parity-lab/**` | CMP-ORACLE (not W4-DEP) |
| `test/parity-lab/**` | CMP-REST / CMP-UI |

## Wave 0

No code parallelism. One documentation PR.

## Wave CMP (parallel with 1–4)

After (or even before) W1-FND, CMP-ORACLE + CMP-REST + CMP-UI can characterize **MailDev only**. Exclusive tree `deploy/parity-lab/**` and `test/parity-lab/**`. Do not edit `internal/**`. CMP-SUT waits for the LabMail image.

See [wave-cmp-comparison-lab.md](wave-cmp-comparison-lab.md).

## Wave 1

```text
W1-FND  (module, Makefile, CI skeleton)
   ├── W1-MODEL   (types only)
   ├── W1-ERR     (error catalog)
   ├── W1-CAP     (registry metadata + completeness test)
   └── W1-CFG     (YAML/flags; depends on MODEL)
```

W1-MODEL, W1-ERR, W1-CAP can start as soon as W1-FND lands directory layout. W1-CFG after MODEL.

## Wave 2

All four after `Inbox` interface and `model.Email` exist:

```text
W2-SMTP  internal/smtpd/**
W2-MIME  internal/mime/** testdata/eml/**
W2-SAN   internal/sanitize/**
W2-STORE internal/store/**
W2-RELAY internal/relay/**
```

Do not share files. SMTP may depend on MIME via constructor injection once MIME’s `Parse([]byte) (Email, error)` is on a small interface — SMTP can stub parse in tests until MIME merges.

Recommended merge order: STORE → MIME → SAN → SMTP (SMTP needs parse+store). Development can still be parallel on feature branches.

## Wave 3

```text
W3-APP   internal/app/**     (serial gate)
   ├── W3-REST  internal/control/rest/**
   ├── W3-MCP   internal/control/mcp/**
   ├── W3-WS    internal/control/ws/**
   └── W3-UI    web/** internal/web/**
W3-AUTH  internal/auth/**    (can parallel APP; REST/MCP consume it)
```

REST and MCP must not both edit `internal/capabilities` bindings without coordination: **W3-APP or W3-CAP-BIND** (single owner) adds ServiceMethods; adapters only register HTTP/MCP.

UI may mock REST with MSW until W3-REST exists, but must not merge assuming paths that are not in docs/06.

## Wave 4

```text
W4-DEP     Dockerfile, deploy/compose/   (not deploy/parity-lab/)
W4-PARITY  test/parity/**
W4-OBS     internal/observability/**  (metrics)
W4-LABDOC  docs/12 + examples for lab flags
```

PARITY needs REST+MCP merged. DEP can stub an image that only healthchecks until serve is wired.

## Wave 5

Interop clients can split by language (Go/Python/Node/Java) as separate tasks with exclusive `test/interop/<lang>/` dirs. Release automation (W5-REL) is serial. GA integrator (W5-GA) is serial and last.

## Merge trains

When stacking PRs: later PRs rebase on earlier. Watch CI on the **last** PR then on `main` ([AGENTS.md](../../AGENTS.md) §2.6). Do not merge a later wave onto `main` while wave contracts are still in flux on another branch.

## Conflict hotspots (serialize)

- `cmd/labmaild/main.go` — W1-FND stub, then W4-DEP/W3 wiring owned by a designated integrator task `W3-WIRE`
- `Makefile` — append-only per task; do not reformat unrelated targets
- `docs/waves/00-program-board.md` — status only
- `CHANGELOG.md` — append bullets under Unreleased; do not rewrite others’ bullets
