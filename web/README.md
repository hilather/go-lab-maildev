# LabMail inbox UI

React + TypeScript + Vite (Node **22.14.0**). The UI talks REST only (`/v1`).

Browser auth is `POST /v1/session` (bearer **or** Basic) → HttpOnly `labmail_session` + CSRF in the JSON body / `GET /v1/session` reload recovery. Mutations send `X-LabMail-CSRF`. The token is never written to `localStorage` or `sessionStorage`.

Pages: sign-in, inbox list, message view (text / sandboxed HTML preview / headers / raw / attachments), status, scoped audit, gated reset. Live update uses `EventSource` `GET /v1/events/stream` with a 3s `GET /v1/messages` poll fallback.

There is no Relay, send, outgoing settings, or compose.

`web/go.mod` is a nested-module fence so parent `go test ./...` does not walk `node_modules`. Do not import `github.com/hilather/go-lab-maildev/web` from the parent module. `//go:embed` cannot leave a module, so `make web-build` copies `web/dist` into `internal/web/dist`. The committed fallback is `internal/web/stub`.

```bash
npm --prefix web test
npm --prefix web run typecheck
npm --prefix web run build
```

Dev server proxies `/v1`, `/mcp`, `/email`, and `/healthz` to `http://127.0.0.1:1080`.
