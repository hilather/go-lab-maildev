# Standards and references

Status: Informative
Last reviewed: 2026-08-17

## RFCs (ingest)

| RFC | Use |
| --- | --- |
| RFC 5321 | SMTP dialog we speak as a sink |
| RFC 5322 | Message format |
| RFC 2045–2047 | MIME / encoded-words |
| RFC 6152 | 8BITMIME |
| RFC 6531 | SMTPUTF8 |
| RFC 1870 | SIZE |
| RFC 3207 | STARTTLS |
| RFC 4954 | AUTH |

We are not an RFC 5321 **relay**. MUST NOT become one.

## MCP

- Model Context Protocol spec revision **2026-07-28**
- Official Go SDK (version pinned in go.mod when added)

## Upstream

- [maildev/maildev](https://github.com/maildev/maildev) MIT — 2.2.1 image behavior and 3.0 UI/API/MCP
- mcp-integration-lab mail sink contract
- Sibling appliances: go-lab-dns, go-lab-ldap-mcp, go-lab-tacacs-mcp (parity, MCP pin, CI culture)

## Libraries (intended)

See [implementation-design.md](implementation-design.md) dependency budget.
