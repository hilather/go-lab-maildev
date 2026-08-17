# ADR 0003: Ephemeral inbox and GitOps desired state

Status: Accepted
Date: 2026-08-17
Decisions: D3

## Context

Lab deployments need easy reset and reviewable configuration. Maildev’s `--mail-directory` would create a second source of truth and contradict the family ephemeral invariant (LabDNS cache, TacLab event ring). Captured mail is runtime evidence, not desired state.

## Decision

**D3 — Desired state is YAML; inbox is not.**

- Load one fail-closed `labmail.dev/v1alpha1` document at startup.
- Config revision is a content hash of the canonical spec (secrets as reference paths, never values).
- Message store has its own monotonic `storeGeneration` (insert/delete/wipe/evict only; not mark-read).
- Reset reloads YAML **and** wipes mail (`store.Wipe` is the only epoch bump).
- The service never writes the bootstrap file.
- Spill on tmpfs is still RAM and is wiped on reset/restart. It is not a mail-directory.

## Consequences

- Restart returns to Git-controlled state and an empty inbox.
- Runtime experiments are easy to discard.
- Agents wait with `mail_messages_wait` rather than depending on mail surviving a bounce.
- Multi-replica shared inbox is out of scope.
- Operators who want to keep a captured message must export it before reset.

## Alternatives considered

- Persist `--mail-directory` like maildev: survives accidental restart, but creates a second source of truth. Rejected.
- Embedded database: durable but conflicts with reset and Git ownership.
- Inbox contents setting `drifted`: would make every received message look like config drift. Rejected; `drifted` is `runtimeRevision != bootstrapRevision` only.

## Review triggers

Review this decision when a durable capture requirement is accepted, or when multi-replica inbox sharing is proposed.
