# Deployment

Status: Proposed normative behavior
Owners: Platform, Operations
Last reviewed: 2026-08-20 (SEC-002 originAllowlist sentinels)
Related ADRs: 0001, 0003, 0008

DEP-001 shipped the hardened image, `examples/compose.smoke.yaml`, `examples/labmail.yaml`, and `scripts/test-container.sh`. Ports and image posture stay frozen here. A `v*` tag is refused unless [`.github/workflows/release.yml`](https://github.com/hilather/go-lab-maildev/blob/main/.github/workflows/release.yml) `tag-gate` sees required CI green on that SHA. Current notes: [docs/releases/v1.0.0-rc.2.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.2.md).

## CLI

```text
labmail serve --config=/etc/labmail/config.yaml
              [--smtp-listen ADDR] [--management-listen ADDR|off]
              [--shutdown-timeout 5s] [--pid-file /tmp/labmail.pid]
labmail validate --config=...
labmail canonicalize --config=... [--format yaml|json]
labmail healthcheck --url=http://127.0.0.1:1080/v1/health/ready
labmail version
```

`serve` loads → compile → bind SMTP → bind management → write pid file. Invalid bootstrap does **not** bind SMTP or management.

`SIGTERM`/`SIGINT`: stop SMTP accept, drain sessions (deadline), then HTTP, then `store.Wipe` spill files. `SIGUSR1` unused (no chaos).

`labmail send` is **not** shipped.

This tree: `serve` binds SMTP (Memory inbox, not Null) and management HTTP (`/v1`, `/mcp`, `/email` when enabled, inbox SPA). `spec.listeners.management.tls.enabled` terminates TLS 1.2+ on that listener and sets `Secure` on `labmail_session`. CFG-001 implements `version`, `help`, `validate`, and `canonicalize`. OBS-001 implements `labmail healthcheck --url=…` against `GET /v1/health/ready` (ready = SMTP bound + store initialized + management bound or explicitly off), slog JSON events, and hand-rolled OpenMetrics (`spec.observability.metrics.listen` / `publicPath`). DEP-001 wires `--smtp-listen`, `--management-listen ADDR|off`, `--shutdown-timeout` (default 5s), and `--pid-file`, plus the hardened image, compose smoke, and `scripts/test-container.sh`.

## Hardened container

Dockerfile (LabDNS shape, Go 1.26.6-alpine → scratch):

```
USER 65532:65532
EXPOSE 1025/tcp 1080/tcp
ENTRYPOINT ["/labmail"]
CMD ["serve", "--config=/etc/labmail/config.yaml"]
HEALTHCHECK CMD ["/labmail", "healthcheck", "--url=http://127.0.0.1:1080/v1/health/ready"]
```

UI-001 added `make web-build` (Node **22.14.0**) which copies `web/dist` into `internal/web/dist` for `go:embed`. DEP-001 should add a Node **22.14.0** image stage that runs that copy before `go build`. UI contract (pages, EventSource + 15s watchdog + 3s exclusive poll fallback, no Relay/send/compose): [docs/01-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/01-architecture.md#embedded-operator-ui). `spec.ui.enabled: false` 404s `/` and keeps REST/MCP.

Posture:

- Image `ghcr.io/hilather/labmail` (`:local` for compose builds, digest-pin in GitOps)
- Non-root UID `65532:65532`
- scratch/static, read-only root
- `cap_drop: ALL`, `no-new-privileges:true`
- tmpfs `/tmp` (optional spill under `/tmp/labmail-spill`)
- no shell, no Docker socket
- Container ports stay **1025 / 1080** (not LabDNS `:8080`)

## Reference compose

Copyable smoke file: [examples/compose.smoke.yaml](https://github.com/hilather/go-lab-maildev/blob/main/examples/compose.smoke.yaml). The lab overlay YAML is [examples/labmail.yaml](https://github.com/hilather/go-lab-maildev/blob/main/examples/labmail.yaml) (SWAP-001; `allowLegacyClients: true`). That path is also reserved by DEP-001 for compose-smoke; do not mount this overlay as the smoke config without 0o644 secret files. Preferred stack split: `examples/labmail/bootstrap.yaml` vs smoke `examples/labmail.yaml`. Bind-mounted secrets are **0o644** (UID 65532). Healthcheck is HTTP ready (exec form). Scratch has no `node`; do not probe SMTP TCP.

```yaml
services:
  labmail:
    image: ghcr.io/hilather/labmail:local
    build:
      context: ..
    command: ["serve", "--config=/etc/labmail/config.yaml"]
    ports:
      - "1025:1025/tcp"
      - "1080:1080/tcp"
    volumes:
      - ./labmail.yaml:/etc/labmail/config.yaml:ro
    read_only: true
    tmpfs:
      - /tmp
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true
    user: "65532:65532"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/labmail", "healthcheck", "--url=http://127.0.0.1:1080/v1/health/ready"]
      interval: 10s
      timeout: 3s
      retries: 3
```

`make test-container` (`scripts/test-container.sh`) builds the image, asserts UID `65532`, `CapEff=0`, Apache-2.0 label, exec-form HTTP ready healthcheck, no `/bin/sh` or busybox, read-only root, then delivers SMTP and lists `/v1/messages`. It parses `examples/compose.smoke.yaml` with `docker compose config` when the plugin is present.

## Integration-lab compose fragment

Service name stays `maildev` (D15). Healthcheck plane change: today’s maildev probe is SMTP TCP via `node -e connect(1025)`. Scratch has no `node`; ready becomes HTTP `/v1/health/ready` (which still requires the SMTP listener bound). `depends_on` / start_period stay 3s / 12 retries.

```yaml
  maildev:   # service name kept for one release so depends_on / docs survive
    image: ghcr.io/hilather/labmail:<pin>
    command: ["serve", "--config=/etc/labmail/config.yaml"]
    networks: [default]
    ports:
      - "${MAILDEV_SMTP_PORT:-1025}:1025/tcp"
      - "${MAILDEV_WEB_PORT:-1080}:1080/tcp"
    volumes:
      - ${MCPLAB_PROFILE_DIR:-./profiles/default}/labmail/bootstrap.yaml:/etc/labmail/config.yaml:ro
      - ./secrets/labmail-token:/run/secrets/labmail-token:ro
      - ./secrets/maildev-web-password:/run/secrets/maildev-web-password:ro
    # Do not leave MAILDEV_ARGS / MAILDEV_WEB_USER / MAILDEV_WEB_PASS
    # here. `environment: {}` does not clear operator env — delete the
    # keys from the service definition entirely.
    # Bind-mounted secrets must be 0o644 (UID 65532).
    read_only: true
    tmpfs: ["/tmp"]
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    user: "65532:65532"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/labmail", "healthcheck", "--url=http://127.0.0.1:1080/v1/health/ready"]
      interval: 5s
      timeout: 3s
      retries: 12
      start_period: 3s
```

Full swap bill of materials: [docs/13-integration-lab-swap.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/13-integration-lab-swap.md).

## Origin allowlist cookbook

Hashed SPA JS/CSS send `Origin`. Empty `originAllowlist` 403s any non-loopback Origin (`forbidden` / `origin is not allowed`). Loopback Origins are already allowed. There is no CORS success path in 1.0 (`OPTIONS` stays `403` `CORS is disabled`). YAML edit + reset or restart (reset wipes the inbox). [ADR 0008](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0008-origin-policy-escape-hatches.md). Worked remote-dev file: [examples/labmail.origin-dev.yaml](https://github.com/hilather/go-lab-maildev/blob/main/examples/labmail.origin-dev.yaml).

| Scenario | Browser Origin | What to do | Auth |
|---|---|---|---|
| Local `labmail serve` + browser `http://127.0.0.1:1080` | loopback | Nothing. | default `bearer_and_basic` |
| In-tree Vite `npm --prefix web run dev` (proxy in `web/vite.config.ts`) | `http://localhost:5173` | Nothing. Loopback. Browser talks to Vite; Vite proxies `/v1` to LabMail. | same |
| SSH tunnel / `kubectl port-forward` so the tab is `http://127.0.0.1:1080` | loopback | Nothing. | same |
| Known stable hostname | `http://devbox:1080` | Exact: `originAllowlist: ["http://devbox:1080"]` | same |
| Published LAN IP, stable | `http://192.168.x.x:1080` | **Prefer exact** `http://192.168.x.x:1080` | same |
| Published LAN IP, DHCP | `http://192.168.x.x:1080` | `originAllowlist: ["private"]` **or** exact IP:port. See footnote. | same |
| Codespaces / preview URL / remote VM public DNS | `https://….app.github.dev` etc. | `originAllowlist: ["*"]` (or the exact Origin if stable) | **Keep bearer_and_basic.** Do not switch to `dev-loopback-unauth`. |
| Remote Vite **with** proxy to LabMail | Vite origin (possibly non-loopback) | List that Vite origin exactly, or `"*"`. Still no CORS (same-origin to Vite). | same |
| Remote Vite **without** proxy (Vite `:5173` calling LabMail `:1080` in the browser) | Vite origin, cross-origin | **Not supported in 1.0.** Use the proxy, a tunnel, or serve the embedded UI from LabMail. Do not enable CORS as a workaround. | — |
| Production / lab overlay `examples/labmail.yaml` | n/a | Leave `originAllowlist: []`. | `bearer_and_basic` |

Footnote — `"private"` is **every** Origin whose host is Go `net.IP.IsPrivate()` (RFC 1918 IPv4 and RFC 4193 ULA only), not “this host’s bind address.” Any such host and port passes the rebinding gate. RFC 6598 CGNAT (`100.64.0.0/10`, including Tailscale `100.x`) is **not** `IsPrivate()`; list those Origins exactly or use `"*"`. `"private"` does **not** allow `devbox.local` or public DNS names. Prefer an exact Origin when the LAN IP is stable.

Do not list default ports unless the browser sends them (`http://host` ≠ `http://host:80`). Quote `"*"` in YAML. Do not pair `"*"` with `dev-loopback-unauth` on a non-loopback bind. Do not ship `"*"` in the image default or `examples/labmail.yaml`.

## Compatibility promise

Compose ports 1025/1080 are the lab contract.
