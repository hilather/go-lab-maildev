# Comparison lab (MailDev oracle + LabMail)

Status: Proposed normative
Last reviewed: 2026-08-17
Related: [parity-plan.md](parity-plan.md), [23-behavior-parity-matrix.md](23-behavior-parity-matrix.md), [ADR 0013](adr/0013-full-maildev-parity-and-comparison-lab.md)

## Purpose

Run **original MailDev** and **LabMail** on the same Docker network, feed them the same SMTP, and assert they behave the same through:

1. **REST** — status codes, JSON bodies (normalized), raw source, attachments, HTML, relay.
2. **UI** — Playwright workflows on both web UIs (not pixel diffs of Angular vs React).

A MailDev capability is **not ported** until the comparison lab has a REST case and a UI case (when the UI exposes it) that pass against the oracle **and** LabMail.

This lab is **not** mcp-integration-lab. It is an in-repo characterization and regression harness. It may enable MailDev outgoing/auto-relay; those connections must terminate on a **captive sink** on the compose network, never a public MTA.

## Oracles

Two oracles cover the surfaces we claim:

| Service | Image / build | REST | UI | Why |
| --- | --- | --- | --- | --- |
| `maildev-v2` | `maildev/maildev:2.2.1` (digest-pinned) | `/email` | AngularJS | mcp-integration-lab contract |
| `maildev-v3` | Build from pinned `maildev/maildev` git SHA (3.0 monorepo) | `/api/email` | React 19 | UI we vendor + extra REST (bulk delete) |
| `labmail` | This repo’s image | both prefixes | Vendored MailDev 3 UI | SUT |
| `relay-sink` | `maildev/maildev:2.2.1` **without** outgoing | `/email` | unused | Captures relayed SMTP |

Pin the v2 digest and the v3 git SHA in `deploy/parity-lab/pins.env`. Bumping a pin is a contract change: re-record characterization goldens.

Until a published MailDev 3 image exists, the compose file **builds** v3 from git (`context: https://github.com/maildev/maildev.git#<sha>` or a vendored submodule under `third_party/maildev` owned by CMP-ORACLE).

## Topology

```text
                    +------------------+
   rest-harness --> |  maildev-v2      | :1025 smtp  :1080 http
   ui-harness   --> |  maildev-v3      | :1025 smtp  :1080 http
                    |  labmail         | :1025 smtp  :1080 http
                    |  relay-sink      | :1025 smtp  :1080 http
                    +------------------+
                         compose network parity-lab
                         NO default egress to public SMTP
```

Internal DNS names are the service names. Host publishes (defaults) so a developer browser can open all three UIs:

| Service | Host SMTP | Host HTTP |
| --- | --- | --- |
| maildev-v2 | 21025 | 21080 |
| maildev-v3 | 31025 | 31080 |
| labmail | 41025 | 41080 |
| relay-sink | 51025 | 51080 |

Harnesses use **container DNS**, not host ports, when running inside compose (`http://maildev-v2:1080`).

## Networks and egress

- Compose network `parity-lab` is internal to the project.
- `relay-sink`, `maildev-v2`, `maildev-v3`, and `labmail` can reach each other on SMTP 1025.
- CI must not pass public smarthost credentials.
- Optional: `egress: deny` / no default route for the mail services except pulling images at build time.
- Auto-relay and manual relay in this lab always use `outgoing-host=relay-sink`, `outgoing-port=1025`, no TLS, no AUTH (sink is open on the isolated net).

## Compose profiles

One file `deploy/parity-lab/compose.yaml` with Compose profiles:

| Profile | What is on | Used by |
| --- | --- | --- |
| `oracle` | maildev-v2, maildev-v3, relay-sink | Characterization before LabMail exists |
| `sut` | + labmail | Dual-run |
| `relay` | Same plus outgoing flags on v2, v3, and labmail | Relay/auto-relay cases |
| `auth` | Web basic + optional SMTP AUTH on all three SUTs | Auth parity |
| `harness` | rest-harness and/or ui-harness one-shots | CI |

Default `make parity-lab-up` = `oracle` + `sut` without relay (outgoing off), matching MailDev defaults and mcp-integration-lab posture.

Overlays (additional compose files, not always-on profiles — auto-relay would otherwise fire during manual Relay tests):

| File | When |
| --- | --- |
| `compose.yaml` | Base: oracles, sut, relay-sink, harness |
| `compose.relay.yaml` | `--outgoing-host relay-sink --outgoing-port 1025` on v2, v3, labmail |
| `compose.autorelay.yaml` | Same plus `--auto-relay` and optional `--auto-relay-rules` |
| `compose.auth.yaml` | `--web-user` / `--web-pass` and `--incoming-user` / `--incoming-pass` (same secrets on all three SUTs) |
| `compose.basepath.yaml` | `--base-pathname /maildev` |
| `compose.directory.yaml` | `--mail-directory /data` + volume |
| `compose.https.yaml` | `--https` + test certs (REST/UI over TLS) |
| `compose.smtps.yaml` | `--incoming-secure` + certs |
| `compose.disableweb.yaml` | `--disable-web` (SMTP still up; HTTP/REST/UI down — MailDev hosts REST on the web server) |

`relay` profile in the table above is shorthand for `compose.yaml` + `compose.relay.yaml`. CI jobs name the files explicitly.

## Service sketches

### maildev-v2

Image `maildev/maildev:2.2.1@sha256:PIN`. Official entrypoint already is `maildev`; extra `command` is flags only:

```text
--smtp 1025 --web 1080 --ip 0.0.0.0
```

Web basic: off for REST comparison unless `compose.auth.yaml` (then the same user/pass on v2, v3, and labmail).

Health: `GET http://127.0.0.1:1080/healthz` from inside the container (may 401 if auth profile is on — then use a TCP check, or hit healthz with credentials; characterize 2.2.1: healthz sits behind the same basic-auth app).

### maildev-v3

Build from the MailDev git SHA in `pins.env` using upstream `Dockerfile` (`ENTRYPOINT ["node", "dist/bin/maildev.js"]`). Same flags as v2.

REST: primary oracle path is `/api/email`. MailDev 3.0-rc claims v2 paths stay compatible — CMP-REST characterization **records** whether `/email` still works; LabMail always serves both.

UI at `/`. MCP may be on (`--mcp`); comparison REST/UI tests do not require it and must not fail if `/mcp` exists.

Health: `GET /api/healthz` (and `/healthz` if present).

### labmail

Build this repo’s `Dockerfile`. Command:

```text
serve --smtp 1025 --web 1080 --insecure-no-auth
```

`--insecure-no-auth` is **comparison-lab only** so REST diffs are not dominated by 401. Production examples still require auth ([ADR 0007](adr/0007-basic-and-bearer-auth.md)). Auth overlay turns it off and sets the same basic credentials as MailDev.

Relay overlay: same `--outgoing-host relay-sink --outgoing-port 1025` as MailDev.

### relay-sink

MailDev 2.2.1, **no** outgoing, **no** web auth, empty inbox at start of each test (`DELETE /email/all`). Tests assert a relayed message appears here. Do not enable auto-relay on the sink.

## Reference compose (implementers copy this tree)

Wave CMP creates these files. Until then this block is the contract.

`deploy/parity-lab/pins.env` (commit real digest/SHA in CMP-ORACLE; placeholders forbidden in CI):

```bash
# Image used for maildev-v2 and relay-sink
MAILDEV_V2_IMAGE=maildev/maildev:2.2.1@sha256:REPLACE_WITH_DIGEST
# Git SHA for maildev-v3 build context
MAILDEV_V3_SHA=REPLACE_WITH_FULL_SHA
```

`deploy/parity-lab/compose.yaml`:

```yaml
name: parity-lab

networks:
  parity-lab: {}

# Do NOT set networks.parity-lab.internal: true — that blocks image pulls and
# git-context builds. Safety is: outgoing-host lint == relay-sink, no public
# MTA credentials, and CI has no MAILDEV_OUTGOING_* secrets.

x-maildev-flags: &maildev-flags
  - --smtp
  - "1025"
  - --web
  - "1080"
  - --ip
  - "0.0.0.0"

services:
  maildev-v2:
    profiles: ["oracle"]
    image: ${MAILDEV_V2_IMAGE}
    command: *maildev-flags
    ports:
      - "21025:1025"
      - "21080:1080"
    networks: [parity-lab]
    restart: "no"

  maildev-v3:
    profiles: ["oracle"]
    build:
      context: https://github.com/maildev/maildev.git#${MAILDEV_V3_SHA}
      dockerfile: Dockerfile
    image: parity-lab/maildev-v3:${MAILDEV_V3_SHA}
    command: *maildev-flags
    ports:
      - "31025:1025"
      - "31080:1080"
    networks: [parity-lab]
    restart: "no"

  labmail:
    profiles: ["sut"]
    build:
      context: ../..
      dockerfile: Dockerfile
    command:
      - serve
      - --smtp
      - "1025"
      - --web
      - "1080"
      - --insecure-no-auth
    environment:
      LABMAIL_INSECURE_NO_AUTH: "true"
    ports:
      - "41025:1025"
      - "41080:1080"
    networks: [parity-lab]
    restart: "no"

  relay-sink:
    profiles: ["oracle"]
    image: ${MAILDEV_V2_IMAGE}
    command: *maildev-flags
    ports:
      - "51025:1025"
      - "51080:1080"
    networks: [parity-lab]
    restart: "no"

  rest-harness:
    profiles: ["harness"]
    build:
      context: ../..
      dockerfile: deploy/parity-lab/Dockerfile.harness
    networks: [parity-lab]
    environment:
      MAILDEV_V2_SMTP: maildev-v2:1025
      MAILDEV_V2_HTTP: http://maildev-v2:1080
      MAILDEV_V3_SMTP: maildev-v3:1025
      MAILDEV_V3_HTTP: http://maildev-v3:1080
      LABMAIL_SMTP: labmail:1025
      LABMAIL_HTTP: http://labmail:1080
      RELAY_SINK_HTTP: http://relay-sink:1080
      RELAY_SINK_SMTP: relay-sink:1025
    depends_on:
      maildev-v2: { condition: service_started }
      maildev-v3: { condition: service_started }
      relay-sink: { condition: service_started }
    # labmail depends_on only when profile sut is on; harness tests skip lm
    # when LABMAIL_HTTP is unreachable.
    command: ["go", "test", "./test/parity-lab/rest/...", "-count=1", "-timeout=15m"]

  ui-harness:
    profiles: ["harness"]
    build:
      context: ../..
      dockerfile: deploy/parity-lab/Dockerfile.playwright
    networks: [parity-lab]
    environment:
      ORACLE_V2_URL: http://maildev-v2:1080
      ORACLE_V3_URL: http://maildev-v3:1080
      LABMAIL_URL: http://labmail:1080
      RELAY_SINK_HTTP: http://relay-sink:1080
    command: ["npx", "playwright", "test", "--config=test/parity-lab/ui/playwright.config.ts"]
```

The harness is a normal service on `parity-lab` so it can DNS all four hosts. If LabMail is not running, tests skip `lm` rows unless `PARITY_LAB_REQUIRE_SUT=1`.

`deploy/parity-lab/compose.relay.yaml`:

```yaml
services:
  maildev-v2:
    command:
      - --smtp
      - "1025"
      - --web
      - "1080"
      - --ip
      - "0.0.0.0"
      - --outgoing-host
      - relay-sink
      - --outgoing-port
      - "1025"
  maildev-v3:
    command:
      - --smtp
      - "1025"
      - --web
      - "1080"
      - --ip
      - "0.0.0.0"
      - --outgoing-host
      - relay-sink
      - --outgoing-port
      - "1025"
  labmail:
    command:
      - serve
      - --smtp
      - "1025"
      - --web
      - "1080"
      - --insecure-no-auth
      - --outgoing-host
      - relay-sink
      - --outgoing-port
      - "1025"
```

`compose.autorelay.yaml` adds `--auto-relay` (and mounts `test/parity-lab/rest/fixtures/auto-relay-rules.json` as `--auto-relay-rules` for Y6/Y7). **Never** combine autorelay with Y1–Y4 in the same container lifetime without wiping state; CI uses a separate job.

Lint (required test in CMP-ORACLE): any `--outgoing-host` / `MAILDEV_OUTGOING_HOST` / YAML `outgoing.host` under `deploy/parity-lab` must equal `relay-sink`. Fail CI if a hostname, IP, or public MX appears.

### rest-harness

Go test binary or `go test ./test/parity-lab/rest/...` run with:

```text
MAILDEV_V2_SMTP=maildev-v2:1025
MAILDEV_V2_HTTP=http://maildev-v2:1080
MAILDEV_V3_SMTP=maildev-v3:1025
MAILDEV_V3_HTTP=http://maildev-v3:1080
LABMAIL_SMTP=labmail:1025
LABMAIL_HTTP=http://labmail:1080
RELAY_SINK_HTTP=http://relay-sink:1080
```

When `oracle` profile only, LabMail env is empty and tests skip SUT rows (characterization mode). CI for wave CMP before W4-DEP runs characterization. After W4-DEP, skip is forbidden.

### ui-harness

Playwright project `test/parity-lab/ui` with three `baseURL`s. Selectors live in `oracles/v2.ts` and `oracles/v3.ts`. LabMail UI uses the **v3 selector map** (same SPA). v2 Angular uses a separate map. Assertions are workflow outcomes, not screenshots (screenshots are artifacts on failure only).

## Test protocol (every matrix row)

1. `DELETE` all mail on v2, v3, labmail, and relay-sink.
2. SMTP the **same fixture bytes** (or the same generator) to each enabled ingest port.
3. REST: poll list until the injected `Message-ID` appears (timeout 10s).
4. Compare per [23-behavior-parity-matrix.md](23-behavior-parity-matrix.md) using [normalization](#json-normalization).
5. UI: drive the same operator steps on each UI; assert visible text / row counts / downloads.
6. Leave containers up; tests must be idempotent via step 1.

## JSON normalization

Do **not** require byte-identical JSON. Compare a canonical form:

| Field | Rule |
| --- | --- |
| `id` | Ignore; correlate on `headers.message-id` or `messageId` |
| `time`, `date` | Presence + parseable; allow clock skew |
| `envelope.host`, `envelope.remoteAddress` | Ignore (container IPs differ) |
| `html` | Compare after sanitizer-stable minify (strip extra whitespace, ignore MailDev vs bluemonday attribute order). Fail on missing user-visible text and on leftover `script` |
| `headers` | Compare keys we set in the fixture; ignore `received`, `x-mailer` if the MUA differs |
| `size` / `sizeHuman` | Allow small deltas; fail if LabMail is 0 or missing attachments |
| `attachments[].checksum` | Must match when MailDev emits one; if only one side has checksum, compare raw bytes via attachment GET |
| `read` | Must match **after the same GET-by-id sequence** (GET marks read) |
| `stream` on v2 attachments | Ignore (v2 leaked stream objects) |

A **normalization snapshot** per fixture is checked in under `test/parity-lab/goldens/` from the oracle. LabMail must match the snapshot after normalization. When MailDev itself is wrong (e.g. leaked `stream`), the golden documents the quirk and LabMail may omit the leak **if** the matrix row says `labmail-cleaner` — still must pass UI/REST functional asserts.

## REST harness shape

```text
test/parity-lab/rest/
  harness.go          # SMTP send, HTTP client, wait, normalize
  normalize.go
  cases_list_test.go
  cases_get_read_test.go
  cases_delete_test.go
  cases_filter_test.go
  cases_html_test.go
  cases_source_download_test.go
  cases_attachment_test.go
  cases_relay_test.go
  cases_autorelay_test.go
  cases_auth_test.go
  cases_config_test.go
  cases_basepath_test.go
  cases_directory_test.go
  cases_https_test.go
  cases_disableweb_test.go
  cases_smtps_test.go
  lint_outgoing_host_test.go
  fixtures/*.eml
  fixtures/auto-relay-rules.json
```

Each test table: `{ name, oracles: []string{"v2","v3","labmail"}, pathFn, fixture, extra }`.

v3 paths use `/api` prefix; v2 and LabMail `/email` (LabMail also asserted on `/api` in the same case).

## UI harness shape

```text
test/parity-lab/ui/
  playwright.config.ts
  oracles/v2.ts
  oracles/v3.ts          # also used for labmail
  specs/inbox.spec.ts
  specs/viewer.spec.ts
  specs/delete.spec.ts
  specs/relay.spec.ts
  specs/live.spec.ts
  specs/auth.spec.ts
  specs/search.spec.ts
  specs/basepath.spec.ts
```

Shared spec, injected oracle adapter:

- `openInbox()`
- `subjects()`
- `openMessage(subject)`
- `tabHtml|Text|Headers|Source()`
- `downloadEml()` / `downloadAttachment(name)`
- `deleteCurrent()` / `deleteAll()`
- `relay({ to?: string })` — no-op skip if oracle UI hidden (must not skip on v2/v3/labmail when relay profile on)
- `waitForSubject(subject)` — live update without reload

## Make targets (wave CMP)

```text
make parity-lab-up          # oracle + sut, outgoing off
make parity-lab-up-relay    # compose.yaml + compose.relay.yaml
make parity-lab-up-autorelay
make parity-lab-down
make test-parity-lab-rest   # go test ./test/parity-lab/rest/...
make test-parity-lab-ui     # playwright
make test-parity-lab        # both; required CI once images exist
make test-parity-lab-oracle # rest+ui against MailDev only (pre-SUT)
make lint-parity-lab-outgoing  # outgoing-host == relay-sink only
```

Suggested Makefile:

```make
PARITY := deploy/parity-lab
COMPOSE := docker compose --env-file $(PARITY)/pins.env -f $(PARITY)/compose.yaml

parity-lab-up:
	$(COMPOSE) --profile oracle --profile sut up -d --wait --remove-orphans

parity-lab-up-relay:
	$(COMPOSE) -f $(PARITY)/compose.relay.yaml --profile oracle --profile sut up -d --wait

parity-lab-up-autorelay:
	$(COMPOSE) -f $(PARITY)/compose.relay.yaml -f $(PARITY)/compose.autorelay.yaml \
	  --profile oracle --profile sut up -d --wait

parity-lab-down:
	$(COMPOSE) --profile oracle --profile sut --profile harness down -v --remove-orphans

lint-parity-lab-outgoing:
	./deploy/parity-lab/lint-outgoing-host.sh

test-parity-lab-oracle:
	$(MAKE) parity-lab-up
	PARITY_LAB_REQUIRE_SUT=0 go test ./test/parity-lab/rest/... -count=1 -timeout=15m
	cd test/parity-lab/ui && npx playwright test

test-parity-lab:
	$(MAKE) lint-parity-lab-outgoing
	$(MAKE) parity-lab-up
	PARITY_LAB_REQUIRE_SUT=1 go test ./test/parity-lab/rest/... -count=1 -timeout=15m
	cd test/parity-lab/ui && npx playwright test
```

`test-parity-lab` is required on PRs that touch SMTP, store, REST, UI, relay, or sanitizer. Oracle-only job can run earlier. Relay and auto-relay are **separate CI matrix cells** that `up` the matching overlay, run Y* cases, then `down`.

Add HTTP or TCP healthchecks on each mail service so `--wait` is meaningful (v3 upstream Dockerfile already has one).

## Safety

- No test may set `outgoing-host` to a non-compose name.
- Lint the compose files: outgoing host must be `relay-sink` if present.
- CI must not contain secrets for a public smarthost.
- mcp-integration-lab examples in *this* repo still omit outgoing ([12-lab-integration.md](12-lab-integration.md)).

## Operator loop

```bash
make parity-lab-up
# v2 UI  http://127.0.0.1:21080
# v3 UI  http://127.0.0.1:31080
# LabMail http://127.0.0.1:41080
swaks --to inbox@lab.test --from sender@lab.test --server 127.0.0.1:41025
make test-parity-lab-rest
make test-parity-lab-ui
```

## Definition of done for a MailDev feature

The row in [23-behavior-parity-matrix.md](23-behavior-parity-matrix.md) is `pass` only when:

- REST case green on every listed oracle **and** LabMail
- UI case green on every listed UI (or `n/a` with a reason, e.g. healthz has no button)
- MCP case green when the capability is `PARITY_REQUIRED` (LabMail only; MailDev 3 MCP is not the oracle)
- Changelog/docs updated
