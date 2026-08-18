# Testing Strategy

Status: Proposed normative behavior
Owners: Quality, SMTP, Control Plane
Last reviewed: 2026-08-18 (SMTP-001b + STORE-001 + STA-001 + API-001 + MCP-001 + OBS-001 + SEC-001 + DEP-001 + UI-001 + SWAP-001 + GA-001)
Related ADRs: 0002, 0004

Every area has regressions. A bug fix starts with a failing test. CI has no optional jobs (LabDNS rule).

## Layers

| Layer | What | Where |
|---|---|---|
| Unit | config decode/unknown fields/reserved names/byte sizes; store caps/wipe/wait/race; auth scopes; domainerr mapping; extract regex; OpenMetrics labels/health | `internal/*` |
| SMTP protocol | 3a: greeting–DATA, SIZE, limits, 452/451 epoch; 3b: AUTH LOGIN/PLAIN transcripts, STARTTLS optional/required + handshake | `internal/smtp/server` with `internal/smtptest`; transcripts in `testdata/smtp` |
| MIME | multipart/alternative, attachments, base64, quoted-printable, broken MIME still stored | `internal/mimeparse` + `testdata/mime` |
| REST contract | OpenAPI, auth 401, list/get/delete/clear/wait/extract, problem+json | `internal/control/rest` |
| Compat | Array + relay 403 + `/healthz` + `testdata/compat` goldens. `TestMaildevScenarioCompat` (401 + Basic + subject + SendMail) | `internal/control/compat` |
| MCP | 2026-07-28 initialize, tools/list, tool call, origin, bearer | `internal/control/mcp` |
| Parity | every `PARITY_REQUIRED` capability: same input types, scopes, errors, side effects | `internal/capabilities` + rest/mcp tests (`make test-parity`) |
| Receive-only | reserved YAML; no relay; import boundary | `internal/config`, `internal/smtp/import_test.go`, `internal/store/import_test.go`, `internal/smtptest/isolation_test.go` |
| Fuzz | SMTP command lines, YAML, MIME, buildinfo | codec + config + mimeparse + committed `testdata/fuzz` corpora |
| Soak | accept N messages, `Wait`, `Wipe` | `internal/perf` (`-soak-n` / `LABMAIL_SOAK_N`; CI default 8) |
| Race | store insert/delete/wait; snapshot swap | `make test-race` |
| Container | non-root, read-only, no caps, healthcheck | `scripts/test-container.sh` (fail-closed until DEP-001) |
| Docs | required files, links, example YAML validates | `make test-docs` |
| Config compat | `testdata/config/valid` + `invalid` | `make test-config-compat` |
| Changelog | user-visible paths require `CHANGELOG.md` | `make test-changelog` |
| Tag gate | notes headings + green required CI on the tag SHA | `.github/workflows/release.yml` |
| Inbox UI | SPA fallback, `ui.enabled: false` 404, CSRF header, empty preview sandbox, no Relay/innerHTML | `internal/web`, `internal/control/rest/spa_test.go`, `make web-test` |

## Required Make targets

Create when first needed; do not skip. Placeholders must fail closed.

```
make format lint generate verify-generated
make test test-race test-fuzz-smoke
make test-parity test-config-compat test-docs
make test-container security-scan test-changelog
```

FND-001 implements `format`, `lint`, `vet`, `build`, `test`, `test-race`, `test-fuzz-smoke`, `test-docs`, and `security-scan`. CFG-001 implements `test-config-compat` and extends `test-fuzz-smoke` with `FuzzDecode`. SMTP-001a adds `testdata/smtp` transcripts, `net/smtp.SendMail` interop, the receive-only import-boundary test, and `FuzzReadLine` on `internal/smtp/codec`. SMTP-001b adds AUTH LOGIN/PLAIN transcripts (`testdata/smtp/auth-login.txt`, `auth-plain.txt`), STARTTLS optional/required fixtures, and `net/smtp` STARTTLS interop. STORE-001 adds `testdata/mime`, store insert/delete/wait/wipe race tests, and `FuzzParse` on `internal/mimeparse`. STA-001 adds snapshot swap, reset-wipes-inbox, in-flight Insert → 451, `replaceStoreCaps` shrink, and audit-on-reset/delete/apply tests. API-001 implements `make generate` / `make verify-generated` (`api/capabilities/v1.json`, `api/openapi/v1.json`) and REST contract tests in `internal/control/rest`. MCP-001 implements Streamable HTTP MCP, `api/mcp/v1.json`, and `make test-parity` (`internal/capabilities` + REST + MCP). COMPAT-001 adds `internal/control/compat` contract tests (array list, relay 403, `/healthz`/`/config`, goldens). OBS-001 adds `api/metrics/v1alpha1.json`, hand-rolled OpenMetrics/label-policy tests, ready-semantics tests, and `labmail healthcheck` tests. SEC-001 adds `internal/auth`, CSRF session, and `TestMaildevScenarioCompat` (`internal/control/compat/scenario_test.go`; goldens in `testdata/compat/`). DEP-001 implements `make test-container` (`scripts/test-container.sh`) and CI job `container-test`. UI-001 adds `make web-test` / `make web-build` (Node **22.14.0**) and `internal/web` embed tests. SWAP-001 adds `examples/labmail.yaml` / MCPJungle / labinfo overlay fixtures and `TestLabOverlayExample`. GA-001 commits fuzz corpora, adds `internal/perf` soak (accept N, wait, wipe), implements `make test-changelog`, and adds Release `tag-gate`.

## Required CI (GA-001)

Jobs: format, lint, unit, race, fuzz-smoke, generated-file, documentation, security-scan, changelog, parity, config-compat, container-test, web. There is no optional or bypassable job. Tag creation is gated by `.github/workflows/release.yml` (`tag-gate`): notes file present, required headings, generated files clean, every required CI job success on the exact tag commit.

## Frozen fixtures present on this tree

- Extract regex coverage for the four RE2 patterns in [docs/03-message-store.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/03-message-store.md) (`TestExtractFrozenRegexes`).
- `TestMaildevScenarioCompat`: unauthenticated `GET /email` → 401, Basic → array containing `subject`, SMTP `SendMail`.
- Compat goldens from `maildev/maildev:2.2.1` for `subject`/`from`/`to`/`id`/`time`/`read` plus one attachment fixture with `fileName` and **no** `stream` (`testdata/compat/`).

## Frozen fixtures to add later

- Implicit SMTPS (`smtp.tls.mode: implicit`) remains 1.1; no 1.0 transcript.
