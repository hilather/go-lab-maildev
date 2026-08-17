# Wave 3 — Control plane and UI

Status: not-started
Dependencies: wave 2 ingest integration green
Serial gate: **W3-APP** then parallel W3-REST, W3-MCP, W3-WS, W3-UI; W3-AUTH parallel with APP; **W3-WIRE** last

Read: [05-control-plane-and-parity.md](../05-control-plane-and-parity.md), [06-rest-api.md](../06-rest-api.md), [07-mcp-api.md](../07-mcp-api.md), [13-frontend.md](../13-frontend.md), ADRs 0003–0007, 0012

## W3-AUTH — Authenticators

Exclusive: `internal/auth/**`

### Goal

Basic and bearer, constant-time, scopes. `Wrap(http.Handler)` skipping health routes.

### Required tests

- [ ] Missing creds 401 + WWW-Authenticate when basic on
- [ ] Wrong password 401
- [ ] Bearer success
- [ ] Basic grants all scopes; bearer honors scope list
- [ ] No secrets in `Error()` strings

---

## W3-APP — Application operations

Exclusive: `internal/app/**`

### Goal

All ServiceMethods used by the registry: list/search/latest/wait/get (mark read)/delete/bulk/deleteAll/readAll/html/source/download/attachment/stats/reset/reload/config/version/status. Wait is context-cancellable with cap. HTML uses sanitizer CID rewrite.

### Required tests

- [ ] Get marks read; list does not
- [ ] Wait returns on insert; timeout → deadline_exceeded
- [ ] Reset clears store not config
- [ ] Delete missing → not_found

No `net/http` or MCP SDK imports.

---

## W3-REST — Dual-prefix HTTP

Dependencies: W3-APP, W3-AUTH
Exclusive: `internal/control/rest/**`

### Goal

All MailDev routes on `/` and `/api`, plus `/v1/*`, problem+json on `/v1`, MailDev error JSON on `/email`. OpenAPI generated or handwritten v0 checked in with `make verify-generated` plan.

### Required tests

- [ ] `/email` vs `/api/email` same list bytes
- [ ] Lab smoke trio: 401, SMTP not here (use store fixture), authed list subject
- [ ] Bulk delete body validation
- [ ] No relay route registered
- [ ] healthz unauthenticated JSON true

---

## W3-MCP — Streamable HTTP + stdio

Dependencies: W3-APP, W3-AUTH
Exclusive: `internal/control/mcp/**`

### Goal

Pin 2026-07-28. Tools/resources/prompts from docs/05–07. Structured content. `allowLegacyClients`. Stdio command shares app. **In-process app, not HTTP.**

### Required tests

- [ ] tools/list names
- [ ] search/get/delete parity fixture vs app (REST comparison can wait for W4-PARITY if REST not merged; at least vs app)
- [ ] unauthorized
- [ ] prompt names include verify-signup-email
- [ ] no maildev_* tool names
- [ ] no send/relay tool

---

## W3-WS — Native WebSocket

Dependencies: W3-APP (subscribe), W3-AUTH
Exclusive: `internal/control/ws/**`

### Goal

`GET /ws` upgrade, JSON `newMail` / `deleteMail`. Auth on Upgrade.

### Required tests

- [ ] Insert emits newMail
- [ ] Delete emits deleteMail
- [ ] Unauthed upgrade 401

---

## W3-UI — Vendor MailDev 3 SPA

Dependencies: REST paths stable (can develop against docs)
Exclusive: `web/**`, `internal/web/**`, `NOTICE`

### Goal

Copy MailDev 3 UI, apply ADR 0005/0012 deltas, build, embed. Playwright: list, open, delete, no Relay button, WS refresh.

### Required tests

- [ ] Unit/component for API client paths `/api`
- [ ] Playwright happy path (or skip-gated with env, but CI must run on main)
- [ ] NOTICE lists MailDev MIT

---

## W3-WIRE — Process wiring

Dependencies: all other W3 tasks
Exclusive: `cmd/labmaild/**` (serve), `internal/server/**` if needed

### Goal

`labmaild serve` starts SMTP + HTTP with MCP, WS, UI, healthcheck subcommand.

### Required tests

- [ ] End-to-end: serve on `:0`, SMTP in, GET `/email` with basic, MCP tools/list with bearer

---

## Wave 3 definition of done

M3. CHANGELOG: REST, MCP, UI. Dual prefix documented as implemented.
