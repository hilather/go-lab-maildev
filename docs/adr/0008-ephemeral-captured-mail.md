# ADR 0008: Ephemeral captured mail

Status: Accepted
Date: 2026-08-17

## Context

Lab desired state is YAML; runtime is wipeable. MailDev optional `--mail-directory` persists `.eml` files. The lab mounts tmpfs.

## Decision

Default store is in-memory. Restart wipes. `mail_state_reset` wipes. Optional directory backend for MailDev flag compatibility, still operator-controlled (tmpfs in lab). Never write captured mail into bootstrap YAML. Inbox full fails SMTP closed (no silent eviction).

## Alternatives considered

- Always persist — fights lab reset semantics.
- Silent eviction like MailDev `maxEmails` — hides test failures.

## Consequences

Long soak tests must raise caps or reset.

## Compatibility impact

MailDev directory reload still exists (no-op on memory).

## Migration

None.

## Test impact

Reset, restart (process test), 452 when full.

## Documentation impact

docs/04, 02.

## Review triggers

Request for durable audit mailbox.
