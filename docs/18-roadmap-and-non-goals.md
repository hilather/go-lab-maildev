# Roadmap and non-goals

Status: Proposed
Last reviewed: 2026-08-17

## Waves (implementation)

See [waves/00-program-board.md](waves/00-program-board.md). This documentation pack is wave 0. Wave CMP (MailDev side-by-side lab) runs in parallel with implementation waves.

## After first GA (not scheduled here)

- IMAP read-only (only with an ADR; easy to become a second product).
- Durable store (SQLite) as explicit opt-in.
- OAuth on `/mcp` (lab phase-1 proxy may sit in front instead).
- Webhooks (optional later; must not imply a public MTA).
- Multi-inbox / multi-tenant.

## Non-goals (do not implement without reversing ADRs)

- Being a general MTA or open relay to the public internet.
- Node compatibility library.
- Socket.IO protocol (UX still compared in the comparison lab).
- AngularJS as LabMail’s UI.
- Clustering the in-memory inbox.
- DKIM signing, greylist, antivirus milter.
