# ADR 0005: Lab static bearer with HTTP Basic compat

Status: Accepted
Date: 2026-08-17
Decisions: D6

## Context

The family (TacLab ADR 0010, LabDNS bearer) uses lab static bearer tokens, not OAuth Protected Resource Metadata. The mcp-integration-lab swap requires HTTP Basic (`MAILDEV_WEB_USER` + `secrets/maildev-web-password`) so existing smoke stays green. MCP clients are bearer-only.

## Decision

**D6 — Lab static bearer is primary; HTTP Basic is an explicit compat authenticator that maps onto the same principal.**

- Tokens: ≥256 bits entropy, compared as SHA-256 digests. File refs only.
- Default container YAML is `bearer_and_basic`.
- Basic: username exact match + constant-time password compare; principal is `tokens[basic.tokenRef]`.
- Failed Basic and failed Bearer both return `401` `unauthenticated` with `WWW-Authenticate: Bearer realm="labmail"` and (if basic enabled) `WWW-Authenticate: Basic realm="labmail"`.
- MCP is bearer-only.
- No `.well-known/oauth-protected-resource`.
- UI session cookie `labmail_session` + `X-LabMail-CSRF` is REST-only.
- `dev-loopback-unauth` is not the image default.

## Consequences

- Smoke `GET /email` with Basic keeps working.
- One verifier, one scope matrix.
- MCPJungle uses `LABMAIL_TOKEN`, not Basic.
- Dual-auth implementation needs shared `auth.Authenticate` and table tests.

## Alternatives considered

- Bearer-only, drop Basic: cleaner, but breaks smoke and labinfo in the same change as the image swap. Rejected.
- OAuth PRM: family exemption for lab static bearer. Rejected.
- Two independent policy engines for Basic vs Bearer: confused-deputy risk. Rejected.

## Review triggers

Review this decision when the lab drops Basic, when OAuth PRM becomes a hard MCP client requirement, or when a second principal model is proposed.
