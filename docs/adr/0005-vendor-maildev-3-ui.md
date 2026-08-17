# ADR 0005: Vendor MailDev 3.0 React UI

Status: Accepted
Date: 2026-08-17

## Context

The user allowed keeping MailDev’s frontend. 2.x is AngularJS. 3.0 is a modern SPA that already speaks the REST we will implement.

## Decision

Vendor MailDev 3 UI under `web/`, Apache-2.0 project with MIT attribution, embed the production build in `labmaild`. Required deltas: native WebSocket, remove Relay (ADR 0012, 0011).

## Alternatives considered

- New first-party UI — higher cost, delayed GA.
- Keep AngularJS — unmaintained.
- UI-less MCP/REST only — worse for operators; lab currently catalogs a web UI.

## Consequences

Node is a **build** dependency, not runtime. Upstream merges need care.

## Compatibility impact

Operator UX stays familiar.

## Migration

Ship SPA in the image.

## Test impact

Playwright workflows; license/NOTICE check.

## Documentation impact

docs/13, NOTICE (wave 3).

## Review triggers

MailDev UI license change; desire for a native console like LabLDAP.
