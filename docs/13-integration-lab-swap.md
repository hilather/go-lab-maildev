# Integration-lab Swap

Status: Proposed normative behavior
Owners: Integration, Platform
Last reviewed: 2026-08-18 (SWAP-001 review)
Related ADRs: 0005, 0006, 0007

This document is the bill of materials for replacing `maildev/maildev:2.2.1` in `mcp-integration-lab` with LabMail. SWAP-001 lands the overlay in **this** repo. The compose/image pin change is a follow-up in that repo after `v1.0.0-rc.1`. DEP-001 image files are stacked later and are not required here.

**Q1 closed:** keep labinfo id `maildev` for the swap release; rename to `labmail` only in a later mcp-integration-lab release (D15). Do **not** rename the id in the same PR as the image pin.

**Q2 closed:** 1.0 includes the inbox UI; GA is not done without PR 12 (UI-001). The swap gate (SMTP + `/email` + Basic) does not wait on the UI, but 1.0 GA does.

## Overlay files in this repo

Copy these into `mcp-integration-lab` at the paths in the BOM. Do not invent a second schema.

| This repo | Lab destination | Role |
|---|---|---|
| [examples/labmail.yaml](https://github.com/hilather/go-lab-maildev/blob/main/examples/labmail.yaml) | `profiles/default/labmail/bootstrap.yaml` | Lab overlay. `allowLegacyClients: true` (D17). Basic user frozen `admin` (`tokenRef: admin`). SMTP AUTH off. Not the DEP-001 compose-smoke file (see collision note). |
| [examples/mcpjungle/servers/labmail.json](https://github.com/hilather/go-lab-maildev/blob/main/examples/mcpjungle/servers/labmail.json) | `profiles/default/mcpjungle/servers/labmail.json` | Filename must match JSON `name` (lab AGENTS.md rule 8). URL is `http://maildev:1080/mcp`. |
| [examples/mcpjungle/groups/integration.json](https://github.com/hilather/go-lab-maildev/blob/main/examples/mcpjungle/groups/integration.json) | `profiles/default/mcpjungle/groups/integration.json` | **Append** `"labmail"` to `included_servers`. Stage 4 is not “add a JSON file” alone. |
| [examples/labinfo/services-maildev.yaml](https://github.com/hilather/go-lab-maildev/blob/main/examples/labinfo/services-maildev.yaml) | merge into `profiles/default/labinfo/services.yaml` | Catalog id stays `maildev`. Adds `/v1` + MCP. SMTP posture unchanged. |

Acceptance twin the lab PR copies by **name**: `TestMaildevScenarioCompat` in `internal/control/compat/scenario_test.go`. Goldens live under `testdata/compat/`. Lab smoke `maildevScenario` must keep the same assertions.

## Current mail sink contract

| Surface | Default | Source |
|---|---|---|
| SMTP ingest | host `:1025` → container `:1025` | `docker-compose.yaml` service `maildev` |
| Web UI + REST | host `:1080` → container `:1080` | same |
| REST list path | `GET /email` | maildev 2.2.1; smoke in `internal/lab/smoke.go` |
| Auth | HTTP Basic `MAILDEV_WEB_USER` / `MAILDEV_WEB_PASS` | `mcplab secrets` → `secrets/maildev-web-password` |
| Receive-only | `internal/maildev` rejects `outgoing-*`, `auto-relay`, `auto-relay-rules` | fail-closed, regression-tested |
| Managed flags | `--smtp`, `--web`, `--web-user`, `--web-pass` not in YAML | lab-owned |
| Profile flags | `profiles/default/maildev/maildev.yaml` `flags:` | optional `incoming-user`/`incoming-pass`, `hide-extensions` |
| Persistence | tmpfs `/tmp`; wiped on restart | compose |
| Healthcheck | TCP connect to SMTP `:1025` (Node one-liner) | compose |
| MCP | **none** (explicit because the image is off-the-shelf) | `docs/architecture.md` |
| Catalog | labinfo id `maildev` | `profiles/default/labinfo/services.yaml` |

Smoke (`internal/lab/smoke.go` `maildevScenario`) is the acceptance test the swap must keep green:

1. `net/smtp.SendMail` to `127.0.0.1:${MAILDEV_SMTP_PORT}` with no auth.
2. Unauthenticated `GET /email` → **401**.
3. Basic-authenticated `GET /email` eventually contains the sent `subject`.

That triplet is `TestMaildevScenarioCompat` here. Stage 4 may add `mail_messages_wait`; it must not rewrite those three assertions.

## Compose fragment

Service name stays `maildev` (D15). **Healthcheck plane change:** today’s maildev probe is SMTP TCP via `node`. Scratch has no `node`; ready becomes HTTP `/v1/health/ready` (which still requires the SMTP listener bound). `depends_on` / start_period stay 3s / 12 retries. Call this out in lab `docs/architecture.md`.

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
    # Bind-mounted secrets must be 0o644 (UID 65532). 0o600 was only
    # safe while MAILDEV_WEB_PASS was env-injected.
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

Image pin and Dockerfile land in DEP-001, then the lab follow-up. This fragment is the compose contract, not an in-tree image.

## Bill of materials

Stage 4 is not “add a JSON file”; token + tool-group registration are mandatory.

| File / surface | Today | Swap must |
|---|---|---|
| `docker-compose.yaml` service `maildev` | `maildev/maildev:2.2.1`, `command: ${MAILDEV_ARGS}`, env `MAILDEV_WEB_*`, SMTP `node` healthcheck, no secret volume | LabMail image, `serve --config`, secret file mounts, HTTP healthcheck, drop `MAILDEV_ARGS` / `MAILDEV_WEB_PASS` keys |
| `internal/lab/runner.go` | `maildev.Args(...)` → `MAILDEV_ARGS`; reads password → `MAILDEV_WEB_PASS` | Stop injecting those env vars; mount bootstrap + token + password files |
| `internal/lab/secrets.go` | `writeTokenIfMissing(secrets/maildev-web-password, 0o600)` (env-injected as `MAILDEV_WEB_PASS`; container never opens the file) | **chmod `maildev-web-password` to 0o644** (existing files too). **Add** `writeTokenIfMissing(secrets/labmail-token, 0o644)` (≥256 bits, same helper/mode as `labdns-token`) and `stageLabinfoCreds` copy. Both files are bind-mounted and read by UID 65532; 0o600 makes `FromSpec` fail-closed and SMTP never binds. 0o600 was only safe while `MAILDEV_WEB_PASS` was injected. |
| `internal/maildev` | Flag renderer + relay reject | One-release shim: translate `flags:` → LabMail YAML **or** delete once no profile uses it. Keep relay-reject tests. |
| `profiles/default/maildev/maildev.yaml` | `flags: {}` | Replaced by `profiles/default/labmail/bootstrap.yaml` (`allowLegacyClients: true`, Basic `tokenRef`, no SMTP AUTH) |
| Flag shim | Rule 11 “everything else is fair game” | See flag matrix below |
| `profiles/default/labinfo/services.yaml` id `maildev` | UI + `/email` + Basic | Keep id; add `/v1` + MCP URL; add bearer credential file; SMTP posture unchanged |
| labinfo compose env | `MAILDEV_SMTP_PORT`, `MAILDEV_WEB_PORT`, `MAILDEV_WEB_USER` | Keep host ports. **Freeze** `MAILDEV_WEB_USER=admin` (YAML does not interpolate it). Add `LABMAIL_TOKEN` only if catalog interpolates it |
| `docker-compose.yaml` `registrar` env | `LABDNS_TOKEN`, `LABLDAP_TOKEN`, `LABTACACS_TOKEN`, `LABINFO_TOKEN` | Add `LABMAIL_TOKEN` (same pattern as LabDNS) |
| `internal/lab/register.go` / `smoke.go` | No mail MCP | Interpolate `${LABMAIL_TOKEN}`; optional `mail_messages_wait` smoke |
| `profiles/default/mcpjungle/servers/labmail.json` | missing | `http://maildev:1080/mcp` + `${LABMAIL_TOKEN}` |
| `profiles/default/mcpjungle/groups/integration.json` | `included_servers`: labdns, labldap, labtacacs, labinfo | **Append** `"labmail"` (AGENTS.md rule 8) |
| `AGENTS.md` rule 11 | maildev.yaml flags + `internal/maildev` | Rewrite: LabMail YAML, receive-only structural, Basic + bearer |
| `docs/architecture.md` | “MCP: none (off-the-shelf)” | LabMail MCP `http://maildev:1080/mcp`; healthcheck plane change |
| `CHANGELOG.md` | maildev receive-only | Image swap + MCP |

labinfo catalog updates (id stays `maildev` — D15 / Q1):

- `urls`: Web UI, REST `/email`, **new** REST `/v1`, **new** MCP `http://${LAB_PUBLIC_HOST}:${MAILDEV_WEB_PORT}/mcp`
- `credential`: still Basic user + password file; add bearer token file `secrets/labmail-token` for MCP/gateway
- `connection.parameters.auth` / `tls` still describe the **SMTP** ingest posture (none / not required by default)
- labinfo `name` becomes `Mail sink (LabMail, receive-only)`

### labinfo snippet

Copy from [examples/labinfo/services-maildev.yaml](https://github.com/hilather/go-lab-maildev/blob/main/examples/labinfo/services-maildev.yaml):

```yaml
services:
  - id: maildev
    name: Mail sink (LabMail, receive-only)
    description: Receive-only SMTP sink (LabMail) with a web UI, maildev /email compat, native /v1 REST, and MCP. It never relays or sends mail outward; captured mail is wiped on restart.
    urls:
      - name: Web UI
        url: http://${LAB_PUBLIC_HOST}:${MAILDEV_WEB_PORT}/
      - name: REST API (maildev /email)
        url: http://${LAB_PUBLIC_HOST}:${MAILDEV_WEB_PORT}/email
      - name: REST API (native /v1)
        url: http://${LAB_PUBLIC_HOST}:${MAILDEV_WEB_PORT}/v1
      - name: MCP endpoint
        url: http://${LAB_PUBLIC_HOST}:${MAILDEV_WEB_PORT}/mcp
    note: "SMTP ingest (no auth, no TLS required): ${LAB_PUBLIC_HOST}:${MAILDEV_SMTP_PORT}. Point systems under test at it as their outbound SMTP server."
    credential:
      file: /run/lab-secrets/maildev-web-password
      usage: "HTTP basic auth for the web UI and /email compat, user 'admin' (frozen; LabMail YAML does not interpolate MAILDEV_WEB_USER)"
    connection:
      endpoints:
        - name: SMTP ingest
          protocol: smtp
          address: ${LAB_PUBLIC_HOST}:${MAILDEV_SMTP_PORT}
          note: configure as the outbound SMTP server of the system under test; all mail is captured, none is relayed
        - name: MCP (streamable HTTP)
          protocol: mcp-streamable-http
          address: http://${LAB_PUBLIC_HOST}:${MAILDEV_WEB_PORT}/mcp
          note: bearer only; gateway interpolates LABMAIL_TOKEN
      parameters:
        auth: "none by default (this profile sets smtp.auth.mode=none; no incoming-user/incoming-pass)"
        tls: "not required; plain SMTP is accepted"
        sender_recipient: "any From/To addresses are accepted and captured"
      credentials:
        - name: labmail-token
          file: /run/lab-secrets/labmail-token
          usage: "HTTP header 'Authorization: Bearer <token>' for native /v1 and MCP; on the lab host: secrets/labmail-token"
```

`stageLabinfoCreds` in `internal/lab/secrets.go` must copy `secrets/labmail-token` into `secrets/labinfo-creds/labmail-token` so the catalog file resolves.

### MCPJungle server JSON

Copy from [examples/mcpjungle/servers/labmail.json](https://github.com/hilather/go-lab-maildev/blob/main/examples/mcpjungle/servers/labmail.json). Registrar env must include `LABMAIL_TOKEN` (same pattern as `LABDNS_TOKEN` in `internal/lab/register.go` `loadTokens` + compose `registrar`).

```json
{
  "name": "labmail",
  "transport": "streamable_http",
  "description": "Receive-only SMTP sink (LabMail): captured mail over REST /v1 and /email, wait/extract, plan/apply/reset. Compose service name stays maildev.",
  "url": "http://maildev:1080/mcp",
  "bearer_token": "${LABMAIL_TOKEN}"
}
```

`groups/integration.json` `included_servers` becomes `["labdns", "labldap", "labtacacs", "labinfo", "labmail"]`.

### AGENTS.md rule 11 rewrite

Replace the maildev flag-bag paragraph. Proposed text:

```text
11. **The mail sink never sends mail.** Compose service name and labinfo
    catalog id stay `maildev` for the swap release (rename later, not in
    the image-pin PR). The image is LabMail. Desired state is
    `profiles/<name>/labmail/bootstrap.yaml` (`labmail.dev/v1alpha1`).
    Receive-only is structural in LabMail: no outbound SMTP, reserved-key
    reject, `POST /email/:id/relay` is 403. Do not reintroduce
    `MAILDEV_ARGS` / `MAILDEV_WEB_PASS` injection in `runner.go`. Host
    ports stay `MAILDEV_SMTP_PORT` / `MAILDEV_WEB_PORT`. Web Basic
    username is frozen at `admin` (`MAILDEV_WEB_USER=admin`; LabMail
    YAML does not interpolate that env — changing profile.env alone
    401s smoke). Password and bearer files are
    `secrets/maildev-web-password` and `secrets/labmail-token`, both
    **0o644** so UID 65532 can read the bind-mounts (0o600 was only
    safe while MAILDEV_WEB_PASS was injected). They share one principal
    via `tokenRef`. If a profile must change the Basic user, the lab
    renderer must write `spec.management.auth.basic.username` from
    `MAILDEV_WEB_USER`. `allowLegacyClients: true` is required for
    MCPJungle. Do not add relay/outbound keys. Implicit SMTPS
    (`incoming-secure`) is 1.1; do not silently map it to STARTTLS.
```

Rule 8 still applies: register through `mcpjungle/servers/labmail.json` **and** the integration tool group.

### architecture.md MCP row and healthcheck

Replace the services-table maildev row and the “MCP: none (off-the-shelf)” note.

| Today | After swap |
|---|---|
| `maildev` · Receive-only SMTP sink with web UI/REST · **MCP: none (off-the-shelf; cataloged in labinfo)** · SMTP 1025, web 1080 | `maildev` (LabMail image) · Receive-only SMTP sink with web UI, `/email` compat, `/v1`, MCP · **MCP: `http://maildev:1080/mcp` (bearer; `allowLegacyClients: true`)** · SMTP 1025, web 1080 |

Healthcheck plane: SMTP TCP via `node -e connect(1025)` → HTTP `GET /v1/health/ready` (`labmail healthcheck`). Ready still requires the SMTP listener bound. Interval 5s, timeout 3s, retries 12, start_period 3s stay.

Also rewrite:

- Configuration ownership: `maildev/maildev.yaml` flags → `labmail/bootstrap.yaml`
- Quirks: delete “maildev is intentionally not given an MCP surface”

## Profile flag shim matrix

One-release `internal/maildev` translator. AGENTS.md rule 11 today: after rejecting relay + managed flags, “everything else in maildev's flag list is fair game.” LabMail does **not** preserve that bag. The shim maps, rejects, or ignores:

| maildev flag | Shim |
|---|---|
| `outgoing-*`, `auto-relay*` | **Reject** (unchanged) |
| `smtp`, `web`, `web-user`, `web-pass` | **Reject** (lab-managed) |
| `incoming-user` / `incoming-pass` | Map → `spec.smtp.auth.mode=plain_login` + files (pass must become a file ref; inline secret → reject) |
| `incoming-secure` / `incoming-cert` / `incoming-key` | **Reject.** maildev `--incoming-secure` is implicit SMTPS (TLS-on-accept), not RFC 3207 STARTTLS. Message: `implicit SMTPS is 1.1; do not silently downgrade to STARTTLS`. LabMail `smtp.tls.mode=starttls` is native YAML only. |
| `hide-extensions` | Map → `smtp.hideExtensions` |
| `ip` / `web-ip` | Map → listener `address` (host still compose-mapped) |
| `verbose` / `silent` | Map → `observability.logLevel` |
| `https` + cert/key (web) | Map → `listeners.management.tls` |
| `disable-web` | Map → `ui.enabled: false` (**REST/MCP stay up** — not maildev’s “kill the web server”) |
| `mail-directory` | **Reject** (ephemeral invariant; `internal/maildev_test.go` still renders it today) |
| `base-pathname` | **Reject** (non-goal) |
| `--mcp` | Ignore (MCP always on when management is bound) |
| unknown flag | **Reject** (fail-closed; no passthrough) |

There is **no** maildev 2.2.1 CLI flag for SIZE (`--max-message-size` does not exist). LabMail default `maxMessageBytes: 10MiB` vs maildev’s implicit ~50 MiB smtp-server default is a documented residual, not a shim mapping.

## Rollout in mcp-integration-lab

Feature flag is the image pin, not a runtime flag:

| Stage | Action | Rollback |
|---|---|---|
| 0 | LabMail rc exists; lab still uses `maildev/maildev:2.2.1` | n/a |
| 1 | Add `profiles/default/labmail/bootstrap.yaml`; keep old `maildev.yaml` unused | revert files |
| 2 | Point compose service `maildev` at LabMail image; keep service name; keep ports; keep Basic user `admin`; stop `MAILDEV_ARGS` / `MAILDEV_WEB_PASS` injection in `runner.go`; add `secrets/labmail-token` at **0o644**; chmod `maildev-web-password` to **0o644**; HTTP healthcheck | pin image back to `maildev/maildev:2.2.1`, restore `MAILDEV_ARGS` |
| 3 | `make smoke` — existing `maildevScenario` must pass unchanged in assertions (`TestMaildevScenarioCompat` twin in this repo) | same |
| 4 | Register MCP: `servers/labmail.json` **and** `groups/integration.json` + `LABMAIL_TOKEN` in registrar env; `allowLegacyClients: true`; extend smoke with `mail_messages_wait` | un-register server JSON + drop group entry |
| 5 | Delete `MAILDEV_ARGS` renderer; rewrite AGENTS.md rule 11 and `docs/architecture.md` “MCP: none” | |

No dual-running of Node maildev and LabMail on the same ports.

LabMail is stateless. Rolling back is an image + command-line revert. Captured mail is lost on any restart either way.

## DEP-001 path collision (`examples/labmail.yaml`)

Design assigned `examples/labmail.yaml` to **both** SWAP-001 (this lab overlay: `allowLegacyClients: true` + `/run/secrets/*` refs) and DEP-001 (`examples/compose.smoke.yaml` mounts `./labmail.yaml` without those secret files; smoke YAML uses `allowLegacyClients: false` and no token `secretFile`s). This branch is stacked on SEC-001, not DEP-001; the collision is latent until stack assembly.

On stack:

- Keep **this** file as the lab overlay (copy target `profiles/default/labmail/bootstrap.yaml`).
- DEP-001 must **not** mount this file as the compose-smoke config unless `test-container.sh` mints `labmail-token` + `maildev-web-password` at 0o644 and compose mounts them. `LoadFile` succeeds without those files; `labmail serve` / `FromSpec` does not. DEP-001 `TestExampleAndContainerYAML` only `LoadFile`s and will not catch that serve failure.
- Preferred split: move the lab overlay to `examples/labmail/bootstrap.yaml` and leave `examples/labmail.yaml` for DEP-001 smoke. Do that on the stack PR, not by inventing a second schema here.

## SWAP-001 checklist (in this repo)

Must name:

- `TestMaildevScenarioCompat` (`internal/control/compat/scenario_test.go`; lab copies the three assertions, not the goldens)
- `runner.go` / `secrets.go` (bind-mounted secrets **0o644**)
- registrar `LABMAIL_TOKEN`
- `integration.json`
- AGENTS.md rule 11 (`MAILDEV_WEB_USER` frozen `admin`; 0o644)
- architecture MCP row
- healthcheck plane change

Shipped here:

- Full file-level BOM above
- `examples/labmail.yaml` with `allowLegacyClients: true` (lab overlay; DEP-001 collision noted)
- labinfo snippet (`examples/labinfo/services-maildev.yaml`)
- `examples/mcpjungle/servers/labmail.json` + group append
- Q1 (catalog id `maildev`) and Q2 (UI required for GA) recorded

Not in this PR: Dockerfile, compose smoke image, or the mcp-integration-lab pin change (DEP-001 + lab follow-up).
