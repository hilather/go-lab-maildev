# Program Board

Status: in-progress (SMTP-001a next)
Last reviewed: 2026-08-17 (FND-001)

Work packages match LabMail 1.0 design PRs 1–14. The numbered pack under `docs/` is the source of truth.

## Work packages

| Order | Task | ID | Depends on | Primary output | Status |
|---:|---|---|---|---|---|
| 1 | Repository foundation | FND-001 | None | Go module, CI, Makefile, design pack, stub CLI | done |
| 2 | Domain model and fail-closed YAML | CFG-001 | FND-001 | `labmail.dev/v1alpha1`, reserved-key reject, revisions | done |
| 3a | In-tree SMTP core (no AUTH/TLS) | SMTP-001a | CFG-001 | Greeting, HELO/MAIL/RCPT/DATA/RSET/QUIT, limits, SendMail | not-started |
| 3b | SMTP AUTH and STARTTLS | SMTP-001b | SMTP-001a | AUTH PLAIN/LOGIN transcript, STARTTLS; `implicit` stays reject | not-started |
| 4 | MIME parse and bounded store | STORE-001 | SMTP-001a | MIME adapter, ULID inbox, caps, wait, wipe, epoch | not-started |
| 5 | Application service, snapshot, reset | STA-001 | CFG-001, STORE-001 | `app.Service`, config snapshot, reset wipes inbox | not-started |
| 6 | REST `/v1` and OpenAPI | API-001 | STA-001 | Native REST except UI/session; problem+json; wait/extract | not-started |
| 7 | maildev 2.2.1 compat adapter | COMPAT-001 | API-001 | `/email`, `/healthz`, `/config`; relay is 403 | not-started |
| 8 | MCP Streamable HTTP and parity | MCP-001 | API-001 | `mail_*` tools, `labmail://` resources, `make test-parity` | not-started |
| 9 | Auth, Basic compat, audit | SEC-001 | API-001, COMPAT-001, MCP-001 | Bearer + Basic, CSRF session, `TestMaildevScenarioCompat` | not-started |
| 10 | Observability | OBS-001 | SMTP-001a, API-001 | slog events, hand-rolled OpenMetrics, ready semantics | not-started |
| 11 | CLI completion, Dockerfile, compose | DEP-001 | SMTP-001a, API-001, OBS-001 | Hardened image, compose contract, healthcheck | not-started |
| 12 | Embedded inbox UI | UI-001 | API-001, SEC-001 | Sandboxed inbox SPA; **required for GA / 1.0** | not-started |
| 13 | Integration-lab swap contract | SWAP-001 | COMPAT-001, MCP-001, SEC-001, DEP-001 | Docs + examples for mcp-integration-lab | not-started |
| 14 | GA hardening | GA-001 | PRs 1–13 (3b for AUTH/STARTTLS claims) | Fuzz, soak, release notes, known limitations | not-started |

## Parallelization

- Prefer PR 2 before 3a. 3b can overlap with PR 4/5 after 3a.
- PR 7 (compat) and PR 8 (MCP) can proceed in parallel after PR 6.
- PR 10 can start as soon as SMTP + REST emit hooks.
- PR 12 (UI) is **required for GA / 1.0**. It must not block the SMTP/compat swap gate; if schedule slips, rc.1 may be API-complete with UI tracked, but 1.0 GA is not done without PR 12.

## Milestones

### M0: Contracts compile

- FND-001 and CFG-001 complete.
- ADRs accepted.
- Schema and semantic test fixtures exist.
- CI runs formatting, lint, unit, and docs checks.

### M1: SMTP usable without control plane

- SMTP-001a and STORE-001 complete.
- `net/smtp.SendMail` against localhost works. Inbox is bounded and wipeable.

### M2: Agent-controllable

- STA-001, API-001, MCP-001, and parity tests complete.
- Plan/apply/export/reset and `mail_messages_wait` work through both transports.

### M3: Swap-gate

- COMPAT-001, SEC-001, OBS-001, DEP-001 complete.
- `TestMaildevScenarioCompat` (401 + Basic + subject) is green.
- Hardened image and HTTP ready healthcheck exist.

### M4: Deployable release candidate

- UI-001, SWAP-001, GA-001 complete.
- Documentation is current.
- Inbox UI ships in 1.0.

### M5: GA

- GA-001 acceptance review passes.
- All required CI must pass on the **tag** commit.
- Residual limitations match [docs/01-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/01-architecture.md#residual-limitations-10).

## Frozen product decisions

| ID | Decision |
|---|---|
| Q1 | labinfo id stays `maildev` for the swap release; rename only in a later mcp-integration-lab release. |
| Q2 | 1.0 includes the inbox UI; GA is not done without PR 12. |

## Cross-cutting blockers

The coordinator must stop dependent work when any of these are unstable:

- Canonical IDs and names.
- Configuration schema source.
- Capability registry API.
- Domain error shape.
- Store epoch / generation contract.
- Supported MCP protocol version.
- Receive-only import boundary.
