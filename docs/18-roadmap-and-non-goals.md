# Roadmap and non-goals

Status: Proposed
Last reviewed: 2026-08-17

## Waves (implementation)

See [waves/00-program-board.md](waves/00-program-board.md). This documentation pack is wave 0.

## After first GA (not scheduled here)

- IMAP read-only (only with an ADR; easy to become a second product).
- Durable store (SQLite) as explicit opt-in.
- OAuth on `/mcp` (lab phase-1 proxy may sit in front instead).
- Webhooks (still must not send SMTP).
- Multi-inbox / multi-tenant.

## Non-goals (do not implement without reversing ADRs)

- Outbound relay / auto-relay / smarthost.
- Being a general MTA.
- Node compatibility library.
- Socket.IO protocol.
- AngularJS UI.
- Clustering the in-memory inbox.
- DKIM signing, greylist, antivirus milter.
