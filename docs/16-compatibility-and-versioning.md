# Compatibility and versioning

Status: Proposed
Last reviewed: 2026-08-17

## MailDev compatibility promise (GA)

| Client | Promise |
| --- | --- |
| mcp-integration-lab smoke (2.2.1 paths) | `GET /email` array, basic auth, SMTP 1025 |
| MailDev 3 UI fork | `/api/*` routes it already calls, minus relay |
| MailDev 3 MCP tool names | **Not** kept; we use `mail_*`. Document mapping in release notes |
| Node `require('maildev')` | No |
| Relay REST | No (404) |

## Breaking vs compatible

Compatible: add `/v1` routes, add MCP, add fields to JSON (clients must ignore unknown fields).

Breaking: remove `/email`, change mark-read-on-GET, send mail, require AUTH on SMTP by default, change id format.

Default SMTP remains **anonymous**. Default HTTP **requires** auth when password/token files exist; refuse to start with open management listener without an explicit insecure flag.

## Config apiVersion

`labmail.dev/v1alpha1` until GA, then `v1`. Unknown fields error. Breaking YAML requires a new apiVersion and migration notes.

## MCP tools

Renaming a tool is breaking. Add a new name and deprecate in notes; do not silently rename.

## MCP protocol

Add versions only after tests. List supported versions in `/v1/version`.
