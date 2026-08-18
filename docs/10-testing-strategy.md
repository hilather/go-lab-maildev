# Testing Strategy

Status: Proposed normative behavior
Owners: Quality, SMTP, Control Plane
Last reviewed: 2026-08-17 (SMTP-001b)
Related ADRs: 0002, 0004

Every area has regressions. A bug fix starts with a failing test. CI has no optional jobs (LabDNS rule).

## Layers

| Layer | What | Where |
|---|---|---|
| Unit | config decode/unknown fields/reserved names/byte sizes; store caps/wipe/wait/race; auth scopes; domainerr mapping; extract regex | `internal/*` |
| SMTP protocol | 3a: greeting–DATA, SIZE, limits, 452/451 epoch; 3b: AUTH LOGIN/PLAIN transcripts, STARTTLS optional/required + handshake | `internal/smtp/server` with `internal/smtptest`; transcripts in `testdata/smtp` |
| MIME | multipart/alternative, attachments, base64, quoted-printable, broken MIME still stored | `internal/mimeparse` + `testdata/mime` |
| REST contract | OpenAPI, auth 401, list/get/delete/clear/wait/extract, problem+json | `internal/control/rest` |
| Compat | PR 7: array + relay 403 + `/healthz` (fake principal). PR 9: `TestMaildevScenarioCompat` (401 + Basic + subject) | `internal/control/compat` |
| MCP | 2026-07-28 initialize, tools/list, tool call, origin, bearer | `internal/control/mcp` |
| Parity | every `PARITY_REQUIRED` capability: same input types, scopes, errors, side effects | `internal/capabilities` + rest/mcp tests (`make test-parity`) |
| Receive-only | reserved YAML; no relay; import boundary | `internal/config`, `internal/smtp/import_test.go`, `internal/store/import_test.go`, `internal/smtptest/isolation_test.go` |
| Fuzz | SMTP command lines, YAML, MIME | codec + config + mimeparse |
| Race | store insert/delete/wait; snapshot swap | `make test-race` |
| Container | non-root, read-only, no caps, healthcheck | `scripts/test-container.sh` |
| Docs | required files, links, example YAML validates | `make test-docs` |
| Config compat | `testdata/config/valid` + `invalid` | `make test-config-compat` |
| Changelog | user-visible paths require `CHANGELOG.md` | `make test-changelog` |

## Required Make targets

Create when first needed; do not skip. Placeholders must fail closed.

```
make format lint generate verify-generated
make test test-race test-fuzz-smoke
make test-parity test-config-compat test-docs
make test-container security-scan test-changelog
```

FND-001 implements `format`, `lint`, `vet`, `build`, `test`, `test-race`, `test-fuzz-smoke`, `test-docs`, and `security-scan`. CFG-001 implements `test-config-compat` and extends `test-fuzz-smoke` with `FuzzDecode`. SMTP-001a adds `testdata/smtp` transcripts, `net/smtp.SendMail` interop, the receive-only import-boundary test, and `FuzzReadLine` on `internal/smtp/codec`. SMTP-001b adds AUTH LOGIN/PLAIN transcripts (`testdata/smtp/auth-login.txt`, `auth-plain.txt`), STARTTLS optional/required fixtures, and `net/smtp` STARTTLS interop. `generate`, `verify-generated`, `test-parity`, `test-container`, and `test-changelog` fail closed until their owning PR.

## Required CI (FND-001)

Jobs: format, lint, unit, documentation. There is no optional or bypassable job. Later PRs add race, fuzz-smoke, generated-file, security-scan, container-test, changelog, parity, and config-compat when those targets first exist.

## Frozen fixtures to add later

- Extract regex goldens for the four RE2 patterns in [docs/03-message-store.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/03-message-store.md).
- `TestMaildevScenarioCompat`: unauthenticated `GET /email` → 401, Basic → array containing `subject`, SMTP `SendMail`.
- Compat goldens from `maildev/maildev:2.2.1` for `subject`/`from`/`to`/`id`/`time`/`read` plus one attachment fixture with `fileName` and **no** `stream`.
