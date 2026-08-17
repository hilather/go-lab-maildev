# go-lab-maildev

A Go-native **receive-only SMTP sink** for laboratory integration testing.

This repository will replace the off-the-shelf
[maildev](https://github.com/maildev/maildev) image used by
[mcp-integration-lab](https://github.com/hilather/mcp-integration-lab).
It belongs with the other first-party lab appliances:

- [LabDNS](https://github.com/hilather/go-lab-dns)
- [LabLDAP](https://github.com/hilather/go-lab-ldap-mcp)
- [TacLab](https://github.com/hilather/go-lab-tacacs-mcp)

Systems under test send mail here. The sink captures it for inspection and
**never relays or sends mail outward**. Runtime mail is ephemeral: restart or
reset wipes captured messages.

Status: **repository initialized**. There is no implementation, image, or
control plane yet.

## Intended lab role

The integration lab currently publishes maildev as:

| Plane | Default host port | Role |
|---|---|---|
| SMTP ingest | 1025 | outbound SMTP target for systems under test |
| Web UI / REST | 1080 | inspect captured mail (`/email`) |

Those listeners, the receive-only posture, and wipe-on-restart semantics are
the starting contract. REST, MCP, YAML, and UI details will be written down
in `docs/` before code lands.

## License

[Apache License 2.0](https://github.com/hilather/go-lab-maildev/blob/main/LICENSE)
