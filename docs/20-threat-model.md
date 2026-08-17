# Threat model

Status: Proposed
Last reviewed: 2026-08-17

## Assets

- Captured message contents (PII, magic links, OTPs from systems under test).
- Management credentials.
- SMTP AUTH credentials (if enabled).
- Process integrity (no pivot to outbound spam).

## Actors

| Actor | Can |
| --- | --- |
| System under test | Submit SMTP |
| Lab operator / agent | REST, MCP, UI after auth |
| Network attacker on lab LAN | Try SMTP and HTTP |
| Malicious captured HTML | XSS against UI users |

## Threats and mitigations

| Threat | Mitigation |
| --- | --- |
| LabMail used as an open relay | No outbound client; config reject; tests |
| XSS via email HTML | Sanitizer + CSP + iframe sandbox in UI |
| Credential theft via logs | Redaction tests |
| Inbox DoS | Size and cardinality limits |
| Auth bypass on /mcp | Same auth middleware as REST; tests |
| Path traversal on attachments | generatedFileName, no `..` |
| SSRF via CID rewrite | Only local attachment URLs, never attacker host |
| Socket.IO extra attack surface | Not implemented |
| Supply chain | Pin modules; `govulncheck`; pin GitHub Actions by SHA |

## Residual risk

See [known-limitations.md](known-limitations.md). Lab exposure of SMTP without AUTH is intentional.
