# mcp-integration-lab integration

Status: Proposed cutover guide (work lands in **both** repos)
Last reviewed: 2026-08-17
Lab repo: [hilather/mcp-integration-lab](https://github.com/hilather/mcp-integration-lab)

## Today

- Compose service `maildev` uses `maildev/maildev:2.2.1`.
- `internal/maildev.Args` renders `--smtp 1025 --web 1080` plus profile flags; rejects relay.
- Smoke: SMTP to `MAILDEV_SMTP_PORT`, unauthenticated `GET /email` is 401, authed list contains the subject.
- labinfo catalogs UI `/` and REST `/email`; SMTP connection block; no MCPJungle server file.
- Architecture doc states maildev has no MCP on purpose.

## LabMail target in that lab

Keep the **service name** `maildev` during cutover if DNS/env churn is painful, or rename to `labmail` in one lab PR (update labinfo id, compose, docs). Prefer rename for honesty, with a compatibility alias if needed.

Required lab PR items (not implemented in this documentation-only change):

1. `internal/lab/vendor.go` pin `go-lab-maildev` like LabDNS.
2. Compose `build.context: ./third_party/go-lab-maildev` (or GHCR pin).
3. `command`: `serve` + existing `MAILDEV_ARGS` **or** mount `profiles/<name>/labmail/config.yaml`.
4. Generate `secrets/labmail-token` in `mcplab secrets`; inject bearer **and** keep web basic password.
5. `profiles/default/mcpjungle/servers/labmail.json` (name matches filename).
6. labinfo: add MCP URL; keep SMTP + `/email`.
7. Smoke: existing REST assertions **plus** `mcpjungle invoke` `mail_emails_search` / `mail_email_get`.
8. Docs/AGENTS: replace “no MCP” with LabMail rules; **keep** the lab orchestrator’s relay-flag rejector (LabMail implements relay for the comparison lab, not for the shared lab profile).
9. Healthcheck: `labmaild healthcheck` instead of `node -e net.connect`.

## Compatibility checklist for this repo

LabMail must pass **before** that lab PR:

- [ ] `GET /email` 401 without auth when basic is set
- [ ] Authed `GET /email` JSON array with `subject`
- [ ] SMTP anonymous on 1025
- [ ] `--smtp` / `--web` / `--hide-extensions` flags work
- [ ] Relay flags **start** the process (parity); integration-lab profiles still omit them
- [ ] `/mcp` tools/list with bearer
- [ ] Read-only rootfs + non-root
- [ ] No Node in the image

## Profile YAML

Short term: keep `maildev/maildev.yaml` flags. LabMail flag overlay consumes them.

Long term: `labmail/config.yaml` apiVersion document, like `labdns/bootstrap.yaml`. LabMail should accept both so the lab can migrate in a second PR.

## Receive-only (integration lab)

Do not remove mcp-integration-lab’s relay-flag rejector. LabMail accepts those flags for MailDev parity; the **profile** still must not set them. Comparison lab lives in this repo (`deploy/parity-lab`), not in mcp-integration-lab.
