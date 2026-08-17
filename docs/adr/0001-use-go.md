# ADR 0001: Use Go for the service

Status: Accepted
Date: 2026-08-17

## Context

LabMail combines a latency-sensitive SMTP receive data plane, an in-memory message store, HTTP management, MCP, immutable runtime config, container deployment, race testing, and fuzzing. The family (LabDNS, LabLDAP, TacLab) is already Go. The repo module is `github.com/hilather/go-lab-maildev`. The toolchain pin is Go 1.26 (D14).

## Decision

Implement the service in Go. Prefer the standard library for HTTP, TLS, concurrency, and SMTP line IO. Isolate the official MCP SDK and (later) `emersion/go-message` behind internal adapters. Do not take a third-party SMTP server library in 1.0 (ADR 0002).

## Consequences

- A single static binary is easy to deploy as `ghcr.io/hilather/labmail` (scratch, UID 65532).
- Go concurrency and context cancellation fit SMTP session caps and store waiters.
- Race detection and fuzzing support hardening.
- Contributors must follow Go memory, cancellation, and error-handling discipline.
- The family CI/Make/docs shape can be copied rather than invented.

## Alternatives considered

- Rust: strong safety and performance, but higher implementation complexity for the initial team and a break from the family.
- Keep wrapping Node maildev: rejected by the repo README and family invariants (no MCP, receive-only is a flag filter).
- Fork maildev v3 (TypeScript): different language, relay still exists, not LabDNS-shaped.
- Replace maildev with Mailpit or MailHog: still off-the-shelf (relay/release features, no `labmail.dev/v1alpha1`, no family capability registry).

## Review triggers

Review this decision when its assumptions no longer hold, a major protocol or library change occurs, or a new requirement conflicts with an invariant.
