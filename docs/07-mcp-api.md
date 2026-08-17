# MCP API

Status: Proposed normative
Last reviewed: 2026-08-17
Target protocol: **2026-07-28**
Related ADRs: 0003, 0006

## Transport

Primary: Streamable HTTP at `spec.mcp.path` (default `/mcp`) on the management listener.

- Prefer **POST-only** if the pinned SDK and protocol allow (TacLab style). If 2026-07-28 requires GET for SSE, implement GET **with the same auth** and session rules the SDK mandates.
- `MCP-Protocol-Version: 2026-07-28`.
- `Authorization: Bearer` and/or HTTP basic (same authenticator as REST).
- Origin validation for browser-like clients.

`allowLegacyClients`: when true, accept older MCPJungle clients that do not send the pin (lab default true). When false, mismatch is a protocol error.

Optional: stdio `labmaild mcp` for desktop agents. Same registry. Logs on stderr. **Stdio talks to in-process `app`, not HTTP.**

## Server identity

```text
name: labmail
version: <build version>
```

Capabilities: tools, resources, prompts (GA). Completions optional later.

## Tools

Names are frozen in [05-control-plane-and-parity.md](05-control-plane-and-parity.md).

Tool results:

```text
content: [{ type: "text", text: "<one-line summary>" }]
structuredContent: { ... domain JSON ... }
```

Search default `limit` 20 (MailDev 3). List tool may return summaries without full HTML to reduce context; `mail_email_get` returns full record.

### `mail_email_wait`

```json
{
  "to": "user@lab.test",
  "subject": "optional substring",
  "query": "optional",
  "timeoutSeconds": 5
}
```

Bounded; cancellable via context. For agents that just sent SMTP from another tool.

## Resources

| URI | Body |
| --- | --- |
| `labmail://emails` | list JSON (metadata; may omit huge bodies — same as list tool) |
| `labmail://emails/{id}` | full email JSON |
| `labmail://stats` | stats |
| `labmail://config` | redacted config |
| `labmail://status` | status |
| `labmail://capabilities` | registry |
| `labmail://schema/config` | JSON Schema |

Dynamic `labmail://emails/{id}` is read via `resources/read` even if not all ids are listed.

## Prompts

Keep MailDev 3 prompt names for agent familiarity:

- `verify-signup-email` (arg `email`)
- `check-password-reset` (arg `email`)
- `analyze-email-content` (arg `emailId`, optional `"latest"`)
- `monitor-email-delivery` (args `to`, optional `subject`)

Prompt text should name **LabMail tool names** (`mail_emails_search`, `mail_email_get`, …), not `maildev_*`.

## MCPJungle

Example server JSON for the lab profile:

```json
{
  "name": "labmail",
  "transport": "streamable_http",
  "description": "Receive-only SMTP sink: inspect and delete captured mail. Never relays.",
  "url": "http://labmail:1080/mcp",
  "bearer_token": "${LABMAIL_TOKEN}"
}
```

Compose service name may remain `maildev` during cutover; hostname in the JSON must match the compose service DNS name.

## Explicitly not provided

- Tools that send or relay mail.
- A generic `http.request` or SMTP client tool.
- Proxying to REST.

## Tests

- `tools/list` contains every `PARITY_REQUIRED` tool.
- `tools/call` vs REST parity fixtures.
- Unauthorized: no tool leakage beyond protocol errors.
- Legacy client flag on/off.
- Stdio: one integration test that list/get works without HTTP.
