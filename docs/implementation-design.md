# Implementation design

Status: Proposed
Last reviewed: 2026-08-17
Depends on: [01-architecture.md](01-architecture.md), ADRs 0001–0012

## Runtime composition

`cmd/labmaild` constructs:

1. `config.Load` → validated `model.Config` (relay keys already impossible).
2. `store.New` (memory; optional directory backend implementing the same interface).
3. `smtpd.Server` with `OnMessage` → `app.Ingest`.
4. HTTP server mux:
   - health
   - REST dual prefix
   - MCP
   - WebSocket
   - static UI
   - OpenAPI (optional `/openapi.json`)
5. Graceful shutdown: stop SMTP accept → drain DATA → stop HTTP → flush logs. Inbox is discarded with the process.

## Store interface

```text
type Inbox interface {
    Insert(EmailRecord) (EmailRecord, error)
    Get(id string) (EmailRecord, error)           // does not mark read
    List(ListQuery) (ListResult, error)
    Delete(id string) error
    DeleteMany(ids []string) (DeleteManyResult, error)
    DeleteAll() (int, error)
    MarkRead(id string) error
    MarkAllRead() (int, error)
    Attachment(emailID, filename string) (AttachmentBlob, error)
    Raw(emailID string) ([]byte, error)
    Reset() error
    Stats() Stats
    Subscribe(ch chan Event) (unsubscribe func())
}
```

`app.GetEmail` calls `Get` then `MarkRead` unless `MarkRead=false` is set on the shared command. List does not mark read.

Concurrency: one mutex or sharded mutex; tests under `go test -race`. Bounded `maxMessages`; insert of N+1 drops oldest **or** rejects — **reject new mail at SMTP with 452** when the cap is hit (fail closed, no silent drop). Document the 452 in SMTP semantics.

## SMTP adapter

Use `github.com/emersion/go-smtp` behind `internal/smtpd`. Do not leak `go-smtp` types from the package API.

Session policy:

- EHLO/HELO required.
- MAIL FROM / RCPT TO: accept any syntactically plausible mailbox unless AUTH is required and missing.
- DATA: read with `io.LimitReader(maxMessageSize+1)`.
- Advertise SIZE, 8BITMIME, SMTPUTF8, PIPELINING unless hidden.
- STARTTLS advertised only when TLS is configured and not hidden.
- AUTH PLAIN/LOGIN when incoming credentials configured.
- No VRFY/EXPN useful data (always 252 or 502; pick one, test it, document it). Recommend **502** (fail closed).

Backend `Login` / `Anonymous` follow config.

## MIME adapter

Use `github.com/emersion/go-message` (and `go-message/mail`) behind `internal/mime`. Produce `model.Email`:

- Decode charset to UTF-8 for text and headers.
- Keep raw headers map (lower-case keys for filter parity with MailDev where documented).
- Attachments: metadata + bytes in store (not only on disk).
- `generatedFileName` for safety (sanitize path segments; MailDev uses a generated name).
- Calculated BCC: envelope recipients not present in To/Cc (MailDev `helpers/bcc.ts`).

Malformed messages: still store raw bytes, surface parse warnings on the record, and expose source/download. Do not 5xx SMTP after DATA if we accepted the bytes unless size/limit tripped.

## HTML sanitizer

`internal/sanitize` wraps a HTML policy (recommended: `microcosm-cc/bluemonday` with an email-oriented policy, plus a documented allowlist). Golden tests from MailDev’s sanitizer tests and additional XSS fixtures.

CID rewrite happens at **HTML GET** time (needs request host + base path), not necessarily at ingest. Store original sanitized HTML with `cid:` still present *or* store both; pick one and test. Recommended: store sanitized HTML with cid intact; rewrite in `app.HTML(id, baseURL)`.

## Application commands (minimum)

See the frozen table in [05-control-plane-and-parity.md](05-control-plane-and-parity.md). Groupings:

- Health/version/capabilities/status/config (config is **read-only effective config**, redacted).
- Email list/get/search/latest/wait.
- Email delete / bulk delete / delete all.
- Mark all read.
- HTML / source / download / attachment.
- Inbox stats / reset.
- Reload from directory (no-op success on memory backend).

## HTTP routing

Single `http.ServeMux` or chi/stdlib mux:

```text
GET  /healthz
GET  /v1/health/live
GET  /v1/health/ready
GET  /v1/version
...
# MailDev dual prefix: register each email route twice
GET/DELETE/PATCH/POST  {/api,}/email...
GET  {/api,}/config
GET  {/api,}/reloadMailsFromDirectory
POST /mcp
GET  /mcp          # only if pinned protocol requires it; prefer POST-only like TacLab if SDK allows
GET  /ws
GET  /  and static assets
```

Basic and bearer both accepted on management routes. WebSocket: same auth via first header or `Sec-WebSocket-Protocol` / query is **forbidden for tokens**; use `Authorization` on the HTTP Upgrade request.

## MCP adapter

Official Go SDK behind `internal/control/mcp`. Pin **2026-07-28**. `allow_legacy_clients` default **true** in lab example compose so MCPJungle (`mark3labs/mcp-go` older generation) can connect; default **false** in hardened examples.

Tools return:

- `structuredContent` JSON matching REST bodies where possible
- a short `text` summary for humans

Do not format-only the way MailDev 3 does.

## UI build

`web/` is a fork. Build in CI (`npm ci && npm run build`). `internal/web` embeds `web/dist`. Dockerfile: Node stage then Go stage; runtime image has no Node.

Delta from upstream MailDev UI (must stay documented in [13-frontend.md](13-frontend.md)):

1. Replace Socket.IO client with `WebSocket` to `/ws`.
2. Remove Relay commands and buttons.
3. Keep `/api` client paths (server aliases `/email`).
4. Hide `isOutgoingEnabled` in settings if present.

## CLI

```text
labmaild serve --config /etc/labmail/config.yaml
labmaild serve --smtp 1025 --web 1080 --web-user admin --web-pass-file /run/secrets/web
labmaild healthcheck --smtp 127.0.0.1:1025 --url http://127.0.0.1:1080/v1/health/ready
labmaild version
```

MailDev binary name `maildev` is **not** required; compose `command:` will use `labmaild serve` plus flag overlay so mcp-integration-lab can keep rendering `MAILDEV_ARGS`.

## Testing seams

- `smtpd` tests: net.Dial + textproto against `127.0.0.1:0`.
- `mime` tests: `testdata/eml/*.eml`.
- `app` tests: fake clock, fake id generator (injectable; production uses crypto/rand mapped to `[a-z0-9]{8}`).
- REST/MCP: `httptest` + SDK client against one `app`.
- Parity: same command → compare REST JSON and MCP structured content.
- Container: compose smoke SMTP + `/email` + `/mcp` tools/list.

## Dependency budget (initial)

Allowed with justification in the PR that adds them:

| Library | Use |
| --- | --- |
| `github.com/emersion/go-smtp` | SMTP server |
| `github.com/emersion/go-message` | MIME |
| `github.com/modelcontextprotocol/go-sdk` | MCP |
| `gopkg.in/yaml.v3` | Config |
| HTML sanitizer (bluemonday or equivalent) | Preview |
| `golang.org/x/crypto` | if needed for TLS helpers |
| `golang.org/x/sync` | errgroup |

Reject: Node embed, Socket.IO Go ports unless ADR 0012 is reversed, outbound SMTP clients (`go-sasl` for **server AUTH** is OK).
