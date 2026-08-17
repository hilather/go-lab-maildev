# LabMail (`go-lab-maildev`)

A Go-native **receive-only SMTP sink** for laboratory integration testing.

This repository replaces the off-the-shelf
[maildev](https://github.com/maildev/maildev) image used by
[mcp-integration-lab](https://github.com/hilather/mcp-integration-lab).
It belongs with the other first-party lab appliances:

- [LabDNS](https://github.com/hilather/go-lab-dns)
- [LabLDAP](https://github.com/hilather/go-lab-ldap-mcp)
- [TacLab](https://github.com/hilather/go-lab-tacacs-mcp)

Systems under test send mail here. The sink captures it for inspection through
REST, MCP, and an embedded web UI, and **never relays or sends mail outward**.
Runtime mail is ephemeral: restart or reset wipes captured messages.

Status: **architecture and parity pack**. There is no `labmaild` binary or image
yet. Implementation is split into parallel waves in [`docs/waves/`](docs/waves/00-program-board.md).

## Intended lab role

The integration lab currently publishes MailDev as:

| Plane | Default host port | Role |
|---|---|---|
| SMTP ingest | 1025 | outbound SMTP target for systems under test |
| Web UI / REST | 1080 | inspect captured mail (`/email`) |

LabMail keeps those listeners, the receive-only posture, wipe-on-restart
semantics, and MailDev 2.2.1 REST paths (so lab smoke keeps working). It adds:

- MailDev 3 `/api/*` aliases for the vendored React UI
- Streamable HTTP MCP at `/mcp` with **full REST parity** (not MailDev 3’s subset)
- YAML config plus MailDev CLI/env overlay for cutover
- Bearer auth for MCPJungle, while keeping HTTP basic for the current lab

## Why not run MailDev as-is?

Evaluated in [docs/00-evaluation.md](docs/00-evaluation.md):

- Lab pin is `maildev/maildev:2.2.1` (Node, no MCP).
- Upstream 3.0 adds MCP, but tools cover only search/get/latest/delete/attachment, stdio MCP calls REST over HTTP, and relay still exists.
- The lab must reject `outgoing-*` / `auto-relay*` in the orchestrator today; LabMail rejects them in the appliance too.

## Documentation

Start at [docs/README.md](docs/README.md). Agents must follow [AGENTS.md](AGENTS.md).

| Doc | Contents |
|---|---|
| [Evaluation](docs/00-evaluation.md) | MailDev 2.2.1 vs 3.0 vs lab contract |
| [Architecture](docs/01-architecture.md) | Process, packages, invariants |
| [Parity plan](docs/parity-plan.md) | MailDev × lab × REST/MCP |
| [Wave board](docs/waves/00-program-board.md) | Parallel implementation tasks |
| [Lab cutover](docs/12-lab-integration.md) | mcp-integration-lab PR recipe |

## Agent and release rules

These are mandatory, not optional hygiene:

1. **Regression tests** for every behavior change and bug fix.
2. **Documentation in the same change** (including `CHANGELOG.md` `[Unreleased]`).
3. **Releases** ship curated notes covering **all** high-level changes since the previous tag ([RELEASE-NOTES-TEMPLATE.md](RELEASE-NOTES-TEMPLATE.md)).
4. **After every PR or PR chain**, watch CI until green; on failure, fix the root cause and **harden** so the class of failure cannot recur silently.

## License

[Apache License 2.0](LICENSE).

The planned embedded UI is a fork of MailDev 3.0 (MIT); attribution will live in
`NOTICE` when that code is vendored (wave 3).
