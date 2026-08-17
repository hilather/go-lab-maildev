# Wave 1 — Foundation

Status: not-started
Dependencies: wave 0 merged
Parallel lanes: W1-MODEL, W1-ERR, W1-CAP after W1-FND; W1-CFG after W1-MODEL

Read: [AGENTS.md](../../AGENTS.md), [01-architecture.md](../01-architecture.md), [04-state-and-configuration.md](../04-state-and-configuration.md), [05-control-plane-and-parity.md](../05-control-plane-and-parity.md), [17-error-model.md](../17-error-model.md)

## W1-FND — Repository skeleton

Recommended owner: platform agent
Exclusive: `go.mod`, `go.sum`, `Makefile`, `.github/workflows/ci.yml`, `cmd/labmaild/main.go` (stub), `.gitignore` (extend), `NOTICE` (placeholder)

### Goal

`go test ./...` and a fail-closed CI workflow exist. `labmaild version` prints a dev version.

### Scope

- [ ] Module `github.com/hilather/go-lab-maildev`
- [ ] Pin Go toolchain (match sibling appliances if reasonable; document in README)
- [ ] Makefile targets: `format`, `lint`, `test`, `test-race`, `ci` (even if lint is `go vet` until golangci-lint lands)
- [ ] CI: format, test, race; pin actions by SHA; no optional required jobs
- [ ] Stub `cmd/labmaild` with `version` subcommand
- [ ] Empty `internal/` packages with `doc.go` so layout matches architecture

### Required tests

- [ ] `TestModulePath` or build that `go list` works
- [ ] Version command test

### Acceptance

- PR CI green; `make ci` locally equivalent documented in Makefile comments.

---

## W1-MODEL — Canonical types

Dependencies: W1-FND
Exclusive: `internal/model/**`

### Goal

`Email`, `Address`, `Attachment`, `Envelope`, `ListQuery`, `Stats`, `Config` structs with JSON tags from [03-mail-model.md](../03-mail-model.md) and config from [04-state-and-configuration.md](../04-state-and-configuration.md). No I/O.

### Required tests

- [ ] JSON round-trip golden for a sample email
- [ ] ID charset helper tests `[a-z0-9]{8}`

---

## W1-ERR — Domain errors

Dependencies: W1-FND
Exclusive: `internal/domainerr/**`

### Goal

Codes from [17-error-model.md](../17-error-model.md), `Is` helpers, HTTP status hints, MCP data helper (no MCP import).

### Required tests

- [ ] Table of code → HTTP status
- [ ] Redaction: error detail cannot be constructed to include a provided secret in helpers if we add `Newf` wrappers

---

## W1-CAP — Capability registry

Dependencies: W1-FND
Exclusive: `internal/capabilities/**`

### Goal

Frozen table from [05-control-plane-and-parity.md](../05-control-plane-and-parity.md) as data. Completeness tests: every `PARITY_REQUIRED` row has REST and MCP name fields filled (adapters may still be missing). Fail if `email.relay` exists.

### Required tests

- [ ] `TestNoRelayCapability`
- [ ] `TestParityRequiredHasBothBindings`
- [ ] `TestToolNamesMailPrefix`

---

## W1-CFG — Config load and MailDev overlay

Dependencies: W1-MODEL, W1-ERR
Exclusive: `internal/config/**`, `config/schema/**`, `config/examples/**`

### Goal

Load YAML, env, flags. Reject unknown fields. Reject all relay flags/keys with a message containing `receive-only`. Defaults: SMTP 1025, HTTP 1080, max 50MiB, mcp enabled, allowLegacyClients true in example lab YAML.

### Required tests

- [ ] Each rejected flag (`outgoing-host`, `auto-relay`, …)
- [ ] `MAILDEV_SMTP_PORT` overlay
- [ ] Unknown YAML field errors
- [ ] Example `config/examples/lab.yaml` validates

### Docs

- [ ] Confirm docs/04 flag table matches code (single table test or generated doc later)

---

## Wave 1 definition of done

All five tasks merged. Program board M1. CHANGELOG notes “repository foundation” if user-visible (CI exists).
