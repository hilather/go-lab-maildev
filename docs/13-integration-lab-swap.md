# Integration-lab Swap

Status: Proposed normative behavior
Owners: Integration, Platform
Last reviewed: 2026-08-17 (FND-001)
Related ADRs: 0005, 0006, 0007

This document is the bill of materials for replacing `maildev/maildev:2.2.1` in `mcp-integration-lab` with LabMail. The compose/image pin change is a follow-up in that repo after `v1.0.0-rc.1`. SWAP-001 lands examples in this repo.

**Q1 closed:** keep labinfo id `maildev` for the swap release; rename to `labmail` only in a later mcp-integration-lab release (D15).

**Q2 closed:** 1.0 includes the inbox UI; GA is not done without PR 12.

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

## Compose fragment

Service name stays `maildev` (D15). Healthcheck plane change: today’s maildev probe is SMTP TCP via `node`. Scratch has no `node`; ready becomes HTTP `/v1/health/ready` (which still requires the SMTP listener bound). `depends_on` / start_period stay 3s / 12 retries.

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

## Bill of materials

Stage 4 is not “add a JSON file”; token + tool-group registration are mandatory.

| File / surface | Today | Swap must |
|---|---|---|
| `docker-compose.yaml` service `maildev` | `maildev/maildev:2.2.1`, `command: ${MAILDEV_ARGS}`, env `MAILDEV_WEB_*`, SMTP `node` healthcheck, no secret volume | LabMail image, `serve --config`, secret file mounts, HTTP healthcheck, drop `MAILDEV_ARGS` / `MAILDEV_WEB_PASS` keys |
| `internal/lab/runner.go` | `maildev.Args(...)` → `MAILDEV_ARGS`; reads password → `MAILDEV_WEB_PASS` | Stop injecting those env vars; mount bootstrap + token + password files |
| `internal/lab/secrets.go` | `writeTokenIfMissing(secrets/maildev-web-password)` | Keep that file; **add** `writeTokenIfMissing(secrets/labmail-token)` (≥256 bits) and `stageLabinfoCreds` copy |
| `internal/maildev` | Flag renderer + relay reject | One-release shim: translate `flags:` → LabMail YAML **or** delete once no profile uses it. Keep relay-reject tests. |
| `profiles/default/maildev/maildev.yaml` | `flags: {}` | Replaced by `profiles/default/labmail/bootstrap.yaml` (`allowLegacyClients: true`, Basic `tokenRef`, no SMTP AUTH) |
| Flag shim | Rule 11 “everything else is fair game” | See flag matrix below |
| `profiles/default/labinfo/services.yaml` id `maildev` | UI + `/email` + Basic | Keep id; add `/v1` + MCP URL; add bearer credential file; SMTP posture unchanged |
| labinfo compose env | `MAILDEV_SMTP_PORT`, `MAILDEV_WEB_PORT`, `MAILDEV_WEB_USER` | Keep; add `LABMAIL_TOKEN` only if catalog interpolates it |
| `docker-compose.yaml` `registrar` env | `LABDNS_TOKEN`, `LABLDAP_TOKEN`, `LABTACACS_TOKEN`, `LABINFO_TOKEN` | Add `LABMAIL_TOKEN` (same pattern as LabDNS) |
| `internal/lab/register.go` / `smoke.go` | No mail MCP | Interpolate `${LABMAIL_TOKEN}`; optional `mail_messages_wait` smoke |
| `profiles/default/mcpjungle/servers/labmail.json` | missing | `http://maildev:1080/mcp` + `${LABMAIL_TOKEN}` |
| `profiles/default/mcpjungle/groups/integration.json` | `included_servers`: labdns, labldap, labtacacs, labinfo | **Append** `"labmail"` (AGENTS.md rule 8) |
| `AGENTS.md` rule 11 | maildev.yaml flags + `internal/maildev` | Rewrite: LabMail YAML, receive-only structural, Basic + bearer |
| `docs/architecture.md` | “MCP: none (off-the-shelf)” | LabMail MCP `http://maildev:1080/mcp`; healthcheck plane change |
| `CHANGELOG.md` | maildev receive-only | Image swap + MCP |

labinfo catalog updates (id stays `maildev` — D15):

- `urls`: Web UI, REST `/email`, **new** REST `/v1`, **new** MCP `http://${LAB_PUBLIC_HOST}:${MAILDEV_WEB_PORT}/mcp`
- `credential`: still Basic user + password file; add bearer token file `secrets/labmail-token` for MCP/gateway
- `connection.parameters.auth` / `tls` still describe the **SMTP** ingest posture (none / not required by default)
- labinfo `name` becomes `Mail sink (LabMail, receive-only)`

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
| 2 | Point compose service `maildev` at LabMail image; keep service name; keep ports; keep Basic; stop `MAILDEV_ARGS` / `MAILDEV_WEB_PASS` injection in `runner.go`; add `secrets/labmail-token`; HTTP healthcheck | pin image back to `maildev/maildev:2.2.1`, restore `MAILDEV_ARGS` |
| 3 | `make smoke` — existing `maildevScenario` must pass unchanged in assertions (`TestMaildevScenarioCompat` twin in this repo) | same |
| 4 | Register MCP: `servers/labmail.json` **and** `groups/integration.json` + `LABMAIL_TOKEN` in registrar env; `allowLegacyClients: true`; extend smoke with `mail_messages_wait` | un-register server JSON + drop group entry |
| 5 | Delete `MAILDEV_ARGS` renderer; rewrite AGENTS.md rule 11 and `docs/architecture.md` “MCP: none” | |

No dual-running of Node maildev and LabMail on the same ports.

LabMail is stateless. Rolling back is an image + command-line revert. Captured mail is lost on any restart either way.

## SWAP-001 checklist (in this repo)

Must name:

- `TestMaildevScenarioCompat`
- `runner.go` / `secrets.go`
- registrar `LABMAIL_TOKEN`
- `integration.json`
- AGENTS.md rule 11
- architecture MCP row
- healthcheck plane change
