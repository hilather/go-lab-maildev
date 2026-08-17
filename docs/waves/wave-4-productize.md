# Wave 4 — Productize

Status: not-started
Dependencies: W3-WIRE
Parallel: W4-DEP, W4-PARITY, W4-OBS, W4-LABDOC

## W4-DEP — Container and compose

Exclusive: `Dockerfile`, `deploy/compose/**`, `.dockerignore`. Do **not** edit `deploy/parity-lab/**` (wave CMP).

### Goal

Multi-stage image, non-root, read-only friendly, expose 1025/1080, healthcheck CMD. `deploy/compose/compose.yaml` example. MAILDEV_ARGS-style command documented.

### Required tests

- [ ] `make test-container` or `docker build` + run + healthcheck (CI)
- [ ] Compose config validates
- [ ] Image contains no `node` binary

---

## W4-PARITY — REST vs MCP harness

Exclusive: `test/parity/**`, `internal/capabilities` test helpers if needed

### Goal

`go test ./test/parity/...` in-process REST vs MCP. **Live MailDev comparison** is wave CMP (`test/parity-lab`), not this package.

### Required tests

- [ ] List, get (read flag), delete, delete all, search, html, attachment, reset, wait, **relay**
- [ ] Fails CI if a new registry row lacks a fixture (or an explicit skip with reason in ADR)

---

## W4-OBS — Metrics and slog

Exclusive: `internal/observability/**` (and small hooks in smtpd/app via interfaces — prefer counters passed in)

### Goal

JSON slog, Prometheus metrics listed in docs/09. `/metrics` auth decision documented and tested.

### Required tests

- [ ] Auth failure not logging password
- [ ] Message accepted increments counter

---

## W4-LABDOC — Cutover pack

Exclusive: `docs/12-lab-integration.md` (update to “ready”), `deploy/compose/maildev-args.env.example`, maybe `docs/releases/` empty

### Goal

Step-by-step PR recipe for mcp-integration-lab with file paths. Include sample `labmail.json` and labinfo snippet. Do **not** modify the lab repo from this task.

### Required tests

- [ ] `make test-docs` links
- [ ] Example MCP JSON parses

---

## Wave 4 definition of done

M4. A reviewer can run compose and repeat lab smoke REST assertions plus MCP tools/list.
