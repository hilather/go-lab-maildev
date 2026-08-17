# State and configuration

Status: Proposed normative
Last reviewed: 2026-08-17
Related ADRs: 0008, 0010, 0011

## Two kinds of state

| Kind | Source | Lifetime |
| --- | --- | --- |
| Desired listen/auth/limits | YAML + flags + env | Process; restored on restart from files |
| Captured mail | SMTP ingest | Memory (optional directory); **wiped on restart and reset** |

The service never writes the YAML file.

## YAML (`labmail.dev/v1alpha1`)

Illustrative schema (source of truth will be `config/schema/` in wave 1):

```yaml
apiVersion: labmail.dev/v1alpha1
kind: LabMail
metadata:
  name: lab
spec:
  smtp:
    listen: ":1025"
    hostname: labmail.local
    hideExtensions: []
    maxMessageSize: 52428800
    maxRecipients: 100
    maxConnections: 64
    auth:
      username: ""          # empty = anonymous
      passwordFile: ""
    tls:
      mode: off             # off | starttls | implicit
      certFile: ""
      keyFile: ""
  http:
    listen: ":1080"
    basePath: ""
    disableUI: false
    tls:
      enabled: false
      certFile: ""
      keyFile: ""
    auth:
      basic:
        username: admin
        passwordFile: /run/secrets/web-password
      bearer:
        tokenFile: /run/secrets/token
  store:
    backend: memory         # memory | directory
    directory: ""
    maxMessages: 10000
  mcp:
    enabled: true
    path: /mcp
    allowLegacyClients: true
    protocolVersion: "2026-07-28"
  log:
    verbose: false
    silent: false
    logMailContents: false  # if true, still redacts AUTH
```

Unknown fields: error. Relay-shaped keys (`outgoing`, `autoRelay`, `relay`): error with message that this is a receive-only sink.

## MailDev flag overlay

Accepted (non-exhaustive; wave 1 owns the full table + tests):

| Flag | Env | Maps to |
| --- | --- | --- |
| `-s/--smtp` | `MAILDEV_SMTP_PORT` | `spec.smtp.listen` port |
| `-w/--web` | `MAILDEV_WEB_PORT` | `spec.http.listen` port |
| `--ip` | `MAILDEV_IP` | SMTP bind |
| `--web-ip` | `MAILDEV_WEB_IP` | HTTP bind |
| `--incoming-user` | `MAILDEV_INCOMING_USER` | SMTP AUTH |
| `--incoming-pass` | `MAILDEV_INCOMING_PASS` | SMTP AUTH (prefer file in YAML) |
| `--incoming-secure` | | TLS implicit |
| `--incoming-cert/--incoming-key` | | TLS files |
| `--hide-extensions` | `MAILDEV_HIDE_EXTENSIONS` | |
| `--mail-directory` | `MAILDEV_MAIL_DIRECTORY` | directory backend |
| `--web-user/--web-pass` | `MAILDEV_WEB_USER` / `MAILDEV_WEB_PASS` | basic auth |
| `--base-pathname` | `MAILDEV_BASE_PATHNAME` | |
| `--disable-web` | | `disableUI` |
| `--https` + key/cert | | HTTP TLS |
| `-v/--verbose`, `--silent`, `--log-mail-contents` | | log |
| `--mcp` | | mcp.enabled (LabMail default **true**) |
| `--config` | | YAML path |

Rejected with a clear error (same spirit as mcp-integration-lab `internal/maildev`):

- `--outgoing-host`, `--outgoing-port`, `--outgoing-user`, `--outgoing-pass`, `--outgoing-secure`
- `--auto-relay`, `--auto-relay-rules`

`--web-user` / `--web-pass` remain valid here (the **lab orchestrator** still refuses to put them in profile YAML; it injects env). LabMail itself must accept them for drop-in.

## Environment

Honor MailDev `MAILDEV_*` names used by the lab. Also honor `LABMAIL_TOKEN_FILE`, `LABMAIL_CONFIG`. Process env overrides YAML; flags override env.

## Reset vs restart

| Action | Config | Inbox |
| --- | --- | --- |
| Process restart | Reloaded from files | Empty (memory) or reloaded if directory backend |
| `POST /v1/state:reset` / `mail_state_reset` | Unchanged | Empty |
| Directory backend + reset | Unchanged | Files deleted then empty |

Lab compose uses tmpfs: restart and reset are equivalent for mail.

## Reload from directory

MailDev `GET /reloadMailsFromDirectory` re-reads `.eml` files. LabMail:

- `backend: directory`: rescan, replace inbox from files (does not send mail).
- `backend: memory`: success no-op (`reloaded: 0`).

This endpoint exists for MailDev UI/compat; it is still `PARITY_REQUIRED` on MCP (`mail_store_reload`).

## Secrets

Prefer `*File` fields. Env passwords are allowed for MailDev drop-in. Never print secrets in `--help` examples that look copy-pasteable with real values. Redact in `GET config` (`password` absent, `passwordConfigured: true`).
