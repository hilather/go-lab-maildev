# Release notes: vX.Y.Z

Replace `vX.Y.Z` and every `TODO` before tagging. A raw commit list is not sufficient. This file must describe **all** functionality differences from the previous release tag (or from repository initialization for the first tag).

Compared to: `TODO previous tag or "repository start"`
Tag commit: `TODO sha`
CI: `TODO green run URL`

## Highlights

TODO: three to eight sentences an operator or lab integrator can act on.

## Added

- TODO

## Changed

- TODO

## Fixed

- TODO

## Removed or deprecated

- TODO

## SMTP / ingest

- TODO: greeting, AUTH, TLS, SIZE, advertised extensions, hide-extensions, limits.

## REST

- TODO: paths, schemas, auth, pagination/filter, compatibility aliases (`/email` vs `/api/email`).

## MCP

- TODO: protocol versions, tools, resources, prompts, `allow_legacy_clients`, gateway notes.

## UI

- TODO: vendored MailDev 3 delta, WebSocket events, relay controls (visible iff outgoing enabled).
- TODO: comparison-lab REST/UI results vs MailDev 2.2.1 and 3.0.

## Configuration and CLI

- TODO: YAML schema, MailDev flag overlay, env vars, defaults.

## Security

- TODO: authn/z, redaction, sanitizer, capability drops, secret files.

## Deployment

- TODO: image tags/digests, compose, ports, healthchecks, non-root, read-only rootfs.

## Compatibility and migrations

- TODO: MailDev 2.2.1 / 3.0, mcp-integration-lab cutover, breaking changes.

## Known limitations

- TODO: link `docs/known-limitations.md` and list anything new.

## Verification

- [ ] `CHANGELOG.md` section for this version lists the same high-level delta.
- [ ] Generated OpenAPI, MCP manifest, config schema, and capability map are current.
- [ ] Required CI is green on this exact commit.
- [ ] No leftover `TODO` or template placeholders in this file.
