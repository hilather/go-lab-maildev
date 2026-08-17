# Known limitations (first GA)

Status: Proposed residual list
Last reviewed: 2026-08-17

These are accepted for first GA unless a wave task closes them.

1. **Anonymous SMTP by default.** Anyone who can reach port 1025 can plant mail. Intentional for the lab data plane.
2. **Single process inbox.** No HA, no shared store across replicas.
3. **No IMAP.** Inspection is REST/MCP/UI only.
4. **GET marks read.** MailDev quirk preserved; list does not.
5. **MCP binary cap** may be lower than REST streaming for huge attachments.
6. **Wait timeout** capped (30s). Agents must poll for slower systems.
7. **Directory backend** is optional and still lab-ephemeral if on tmpfs; not a backup product.
8. **MailDev MCP tool names** (`maildev_*`) are not aliased.
9. **Socket.IO clients** will not work; only `/ws`.
10. **Relay HTTP clients** get 404.
11. **HTML sanitizer** will not match DOMPurify byte-for-byte; golden tests define the policy.
12. **Legacy MCP clients** need `allowLegacyClients: true` for MCPJungle until the gateway speaks 2026-07-28.
13. **No OAuth PRM** on `/mcp` (static bearer / basic), same class as TacLab ADR 0010.

When a limitation is removed, delete it here and mention it in release notes.
