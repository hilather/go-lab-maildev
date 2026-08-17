# ADR 0009: Native Go SMTP and MIME adapters

Status: Accepted
Date: 2026-08-17

## Context

MailDev uses Nodemailer `smtp-server` and `mailparser`. A Go rewrite should not shell out to Node.

## Decision

`internal/smtpd` wraps `github.com/emersion/go-smtp`. `internal/mime` wraps `github.com/emersion/go-message`. Library types do not escape those packages. HTML sanitizer is a separate adapter.

## Alternatives considered

- Custom SMTP from `net.Conn` only — more code, more RFC bugs.
- Keep Node SMTP sidecar — not a rewrite.

## Consequences

Need fixture tests to match real clients, not just the library’s self-tests.

## Compatibility impact

EHLO wording and exact reply text may differ; clients care about codes and extensions.

## Migration

None.

## Test impact

Interop with Go, Python, Nodemailer, swaks (wave 5).

## Documentation impact

implementation-design, SMTP semantics.

## Review triggers

Library abandonment; need for a feature it cannot express.
