# Contributing

This is laboratory software: a receive-only SMTP sink with REST, MCP, and an embedded UI. Runtime captured mail is ephemeral. Desired listen/auth/limits live in YAML (with MailDev-compatible flags for lab cutover).

## Before you open a PR

Read [AGENTS.md](AGENTS.md). Then:

```bash
make ci          # once the Makefile exists (wave 1)
```

Until `make ci` exists, run the equivalent format, lint, test, and docs checks the task added.

## Rules that block merge

- Every behavior change ships with regression tests.
- Documentation and `CHANGELOG.md` `[Unreleased]` are updated in the same change.
- REST and MCP stay at parity for public control capabilities.
- The sink never sends mail. Relay/outbound configuration is rejected.
- After push, watch CI until green. Fix root causes and harden; do not retry flakes away.

## Where things belong

| Kind of change | Put it here |
| --- | --- |
| Domain behavior | `internal/app`, `internal/model`, `internal/store` |
| SMTP wire | `internal/smtpd` (library types do not escape the adapter) |
| REST / MCP / WebSocket | `internal/control/*` adapters only |
| UI | `web/` (MailDev 3 fork) + `internal/web` embed |
| Config schema / examples | `config/` |
| Normative design | `docs/` and `docs/adr/` |
| Implementation tasks | `docs/waves/` |

## Releases

Promote `[Unreleased]` to a dated `CHANGELOG.md` section, write `docs/releases/vX.Y.Z.md` from [RELEASE-NOTES-TEMPLATE.md](RELEASE-NOTES-TEMPLATE.md), and tag only a commit whose required CI is green. See [docs/14-release-engineering.md](docs/14-release-engineering.md).
