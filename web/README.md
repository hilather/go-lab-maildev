# LabMail inbox UI

React + TypeScript + Vite (Node **22.14.0**). The UI talks REST only (`/v1`).

Browser auth is `POST /v1/session` (bearer **or** Basic) → HttpOnly `labmail_session` + CSRF in the JSON body / `GET /v1/session` reload recovery. Mutations send `X-LabMail-CSRF`. The token is never written to `localStorage` or `sessionStorage`.

Pages: sign-in, split inbox on `/` (captured list + inspector: sandboxed HTML / text / raw / headers), status, scoped audit, gated reset. Dark lab chrome (header chips, left rail, unread badge). Selecting a message `GET`s it without `markRead` and then `POST /v1/messages/{id}:read` with CSRF. Live update uses `EventSource` `GET /v1/events/stream` with a 3s `GET /v1/messages` poll fallback.

There is no Relay, send, outgoing settings, or compose.

`web/go.mod` is a nested-module fence so parent `go test ./...` does not walk `node_modules`. Do not import `github.com/hilather/go-lab-maildev/web` from the parent module. `//go:embed` cannot leave a module, so `make web-build` copies `web/dist` into `internal/web/dist`. The committed fallback is `internal/web/stub`.

```bash
npm --prefix web test
npm --prefix web run typecheck
npm --prefix web run build
```

Dev server proxies `/v1`, `/mcp`, `/email`, and `/healthz` to `http://127.0.0.1:1080`.

Loopback Origins (`http://localhost:5173`) are already allowed. Remote Vite needs that origin on `spec.management.originAllowlist` (or `"*"`) **and** the proxy; there is no CORS success path in 1.0. Cookbook: [docs/11-deployment.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/11-deployment.md#origin-allowlist-cookbook).
