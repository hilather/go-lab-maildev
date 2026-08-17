# Mail model

Status: Proposed normative
Last reviewed: 2026-08-17

Canonical types live in `internal/model`. JSON names below are the REST/MCP wire names unless noted.

## Email

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | `[a-z0-9]{8}`, unique in process |
| `time` | string RFC3339 nano | Receive time (injectable clock in tests) |
| `read` | bool | GET-by-id sets true |
| `subject` | string | Decoded |
| `from` | Address[] | |
| `to` | Address[] | |
| `cc` | Address[] | |
| `bcc` | Address[] | Header BCC if present |
| `calculatedBcc` | Address[] | Envelope minus To/Cc |
| `date` | string RFC3339 | From Date header if parseable |
| `text` | string | Plain body |
| `html` | string | Sanitized HTML (may still contain `cid:`) |
| `headers` | object | Lower-case keys; values string or string[] |
| `priority` | `normal` \| `high` \| `low` | |
| `attachments` | Attachment[] | Metadata only on list/get |
| `envelope` | Envelope | |
| `size` | int | Raw bytes |
| `sizeHuman` | string | Display |
| `messageId` | string | From Message-ID header, optional |
| `inReplyTo` | string | optional |
| `parseWarnings` | string[] | Malformed MIME; empty if clean |

List endpoints may omit `html` and `text` bodies when `?body=false` (LabMail addition, default **include** for MailDev compatibility). If payloads get large, add a documented `fields=` later via ADR.

## Address

```json
{ "address": "johnny.utah@fbi.gov", "name": "Johnny Utah" }
```

## Envelope

```json
{
  "from": { "address": "angelo.pappas@fbi.gov" },
  "to": [{ "address": "johnny.utah@fbi.gov" }],
  "host": "client.example",
  "remoteAddress": "203.0.113.10"
}
```

v2 samples used `envelope.from` as a bare string. **Emit object form** (v3) and accept that lab smoke only reads `subject`. Add a compatibility test that v2-style clients still can parse if we discover they cannot; do not dual-emit without evidence.

## Attachment

| Field | Type |
| --- | --- |
| `filename` | original, sanitized for display |
| `generatedFileName` | used in URLs |
| `contentType` | |
| `contentDisposition` | `inline` \| `attachment` |
| `contentId` | optional |
| `size` | bytes |

Blob bytes are not in the JSON get body. Fetch via attachment route/tool.

## List query

Shared by REST and MCP:

| Param | Meaning |
| --- | --- |
| `skip` | Offset (MailDev) |
| `limit` | Max results (default 0 = no extra cap for REST list, 20 for search tool) |
| `query` | Substring across subject, from, to, text |
| `from` / `to` / `subject` | Substring (search) |
| dotted keys | Exact match (MailDev filter) |
| `hasAttachment` | bool |
| `isUnread` | bool |
| `since` / `until` | RFC3339 |

Exact dotted filters and substring search can combine; document AND semantics. Tests cover `from.address=`, `headers.x-mailer=`, `read=false`.

## Stats

```json
{
  "emailCount": 0,
  "unreadCount": 0,
  "newestEmail": null,
  "oldestEmail": null
}
```

Timestamps ISO-8601 or null.

## Events

```text
MailReceived { email }   // full metadata; bodies allowed
MailDeleted  { id }
InboxReset   {}
```

## Identifiers

Production generator: cryptographic randomness mapped onto `[a-z0-9]{8}` (not `math/rand`). Collision: retry. Tests inject sequential ids.

Do not use the RFC Message-ID as the inbox id (MailDev does not).
