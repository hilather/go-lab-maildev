# ADR 0007: HTTP basic and bearer

Status: Accepted
Date: 2026-08-17

## Context

MailDev and the lab smoke use HTTP basic on :1080. Sibling appliances and MCPJungle use bearer tokens. Drop-in plus first-class MCP needs both.

## Decision

Management HTTP accepts basic (MailDev env/flags) and/or bearer (token file). Either success authenticates. Basic maps to all scopes; bearer has scopes. Health live/ready/healthz stay unauthenticated. Refuse to start with no auth unless an explicit insecure flag is set.

SMTP AUTH remains a separate data-plane option.

## Alternatives considered

- Bearer only — breaks lab smoke and browser UI convenience.
- Basic only — awkward for MCPJungle (possible but inconsistent).
- OAuth PRM — lab phase-1 proxy; not this appliance’s first GA.

## Consequences

Two authenticators to test. Tokens never in query strings.

## Compatibility impact

Lab basic continues. New token for MCP.

## Migration

`mcplab secrets` grows a mail token.

## Test impact

401 without creds on `/email`; 200 with basic; 200 with bearer; 401 with wrong either.

## Documentation impact

docs/08, 06, 12.

## Review triggers

OAuth requirement.
