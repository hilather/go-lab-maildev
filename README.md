<p align="center">
  <img src="docs/assets/labmail-header.jpg" alt="LabMail — receive-only SMTP for the lab" width="100%">
</p>

<h1 align="center">LabMail</h1>

<p align="center">
  <strong>Receive-only SMTP lab appliance.</strong><br>
  Systems under test deliver mail here. LabMail captures it, indexes it, and
  hands it back over REST, MCP, and an embedded inbox UI.
</p>

<p align="center">
  <a href="https://github.com/hilather/go-lab-maildev/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/hilather/go-lab-maildev/ci.yml?branch=main&label=CI&style=flat-square" alt="CI"></a>
  <a href="https://go.dev/dl/"><img src="https://img.shields.io/github/go-mod/go-version/hilather/go-lab-maildev?label=Go&style=flat-square" alt="Go"></a>
  <a href="https://github.com/hilather/go-lab-maildev/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-1f6feb?style=flat-square" alt="Apache-2.0"></a>
  <img src="https://img.shields.io/badge/status-v1.0.0--rc.2-d4a04a?style=flat-square" alt="v1.0.0-rc.2">
  <img src="https://img.shields.io/badge/posture-receive--only-0d9488?style=flat-square" alt="receive-only">
</p>

<p align="center">
  <code>labmail.dev/v1alpha1</code>
  · binary <code>labmail</code>
  · image <code>ghcr.io/hilather/labmail</code>
  · module <a href="https://github.com/hilather/go-lab-maildev"><code>github.com/hilather/go-lab-maildev</code></a>
</p>

LabMail **never** opens an outbound SMTP session, **never** relays, and **never**
implements `POST /email/:id/relay`. Desired state is one YAML file. The inbox is
ephemeral: restart or reset returns the process to the mounted bootstrap and an
empty store.

It is a lab sink, not a public MTA. Known limits:
[docs/known-limitations.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/known-limitations.md).
Release notes:
[docs/releases/v1.0.0-rc.2.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.2.md).

New here? Start with [START-HERE.md](https://github.com/hilather/go-lab-maildev/blob/main/START-HERE.md).
Architecture, ADRs, and the program board are in [Documentation](#documentation).

---

## Why LabMail

Labs need a mailbox they can **point at**, **inspect**, and **wipe**. LabMail is
that service — and the in-family replacement for the off-the-shelf
[maildev](https://github.com/maildev/maildev) image used by
[mcp-integration-lab](https://github.com/hilather/mcp-integration-lab).

| You need | LabMail does |
|---|---|
| A place for the system under test to send mail | In-tree RFC 5321 SMTP on `:1025` |
| See what arrived | Native `/v1`, maildev `/email` compat, and an embedded inbox SPA |
| Let an agent wait for a message | `POST /v1/messages:wait` and `mail_messages_wait` |
| GitOps the sink | One `labmail.dev/v1alpha1` file; unknown fields and relay keys fail closed |
| Change posture without restarting | `GET /v1/state`, `:validate`, `:export`, `:reset`, and `/v1/changes:plan` / `:apply` |
| Stay receive-only | No outbound SMTP client, no relay config, `POST /email/:id/relay` is always 403 |

Family siblings: [LabDNS](https://github.com/hilather/go-lab-dns) ·
[LabLDAP](https://github.com/hilather/go-lab-ldap-mcp) ·
[TacLab](https://github.com/hilather/go-lab-tacacs-mcp).

### Intended lab role

During the mcp-integration-lab swap the catalog id stays **`maildev`**.

| Plane | Default host port | Role |
|---|---|---|
| SMTP ingest | 1025 | Outbound SMTP target for systems under test |
| Management / UI / REST / MCP | 1080 | Inspect captured mail (`/v1`, `/email`, `POST /mcp`, SPA at `/`) |

---

## Quick start

### 1. Build

```bash
git clone https://github.com/hilather/go-lab-maildev.git
cd go-lab-maildev
go version   # go1.26.x
go build -o bin/labmail ./cmd/labmail
./bin/labmail version
./bin/labmail help
```

Or build the hardened image (non-root UID `65532`, scratch, read-only root,
`cap_drop: ALL`):

```bash
docker build -t ghcr.io/hilather/labmail:local .
```

Compose smoke:
[examples/compose.smoke.yaml](https://github.com/hilather/go-lab-maildev/blob/main/examples/compose.smoke.yaml).

### 2. Write a bootstrap YAML

LabMail loads **one** `labmail.dev/v1alpha1` document. Unknown fields fail
closed. Durations use Go syntax (`30s`, `5m`). Byte sizes use binary units
(`10MiB`, `256KiB`). Secrets are **file references**, never inline values.

The smallest valid file materializes every default:

```yaml
apiVersion: labmail.dev/v1alpha1
kind: LabMail
metadata:
  name: lab-sink
spec: {}
```

That is
[testdata/config/valid/defaults.yaml](https://github.com/hilather/go-lab-maildev/blob/main/testdata/config/valid/defaults.yaml).
For a local inbox you can open in the browser without minting tokens, override
auth to loopback-unauthenticated:

```yaml
apiVersion: labmail.dev/v1alpha1
kind: LabMail
metadata:
  name: local
spec:
  listeners:
    smtp:
      address: "127.0.0.1:1025"
    management:
      address: "127.0.0.1:1080"
      compatEnabled: true
  smtp:
    hostname: labmail.lab
  ui:
    enabled: true
  management:
    auth:
      mode: dev-loopback-unauth
```

Lab and container defaults stay `bearer_and_basic` with tokens pointed at
`secretFile` paths. Full field map, reserved-key list, and revision rules:
[docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md).
JSON Schema:
[api/jsonschema/labmail.dev.v1alpha1.json](https://github.com/hilather/go-lab-maildev/blob/main/api/jsonschema/labmail.dev.v1alpha1.json).
Lab overlay with tokens and MCPJungle knobs:
[examples/labmail.yaml](https://github.com/hilather/go-lab-maildev/blob/main/examples/labmail.yaml).

### 3. Validate, canonicalize, serve

```bash
./bin/labmail validate --config testdata/config/valid/defaults.yaml
./bin/labmail canonicalize --config testdata/config/valid/defaults.yaml
./bin/labmail serve \
  --config testdata/config/valid/defaults.yaml \
  --smtp-listen 127.0.0.1:1025 \
  --management-listen 127.0.0.1:1080
```

`validate` and `canonicalize` load the document fail-closed (unknown fields,
reserved relay keys, bad TLS combinations, unbounded `maxMessageBytes: 0`).
`canonicalize` prints the normalized spec used for the revision hash.

`serve` binds SMTP from the compiled YAML (`--smtp-listen` overrides) and
management HTTP from `spec.listeners.management.address`
(`--management-listen ADDR|off` overrides). On that listener:

| Surface | Path |
|---|---|
| Inbox SPA | `/` (when `spec.ui.enabled` is true) |
| Native REST | `/v1` |
| Streamable HTTP MCP | `POST /mcp` |
| maildev compat | `/email`, `/healthz`, `/config` (when `compatEnabled`, default true) |
| Relay | `POST /email/:id/relay` → **403** `receive_only` |

`--shutdown-timeout` (default 5s) and `--pid-file` are optional. Ready is
SMTP bound + store up. Probe it:

```bash
./bin/labmail healthcheck --url=http://127.0.0.1:1080/v1/health/ready
curl -sS http://127.0.0.1:1080/v1/health/ready
```

Metrics are hand-rolled OpenMetrics on `spec.observability.metrics.listen`
(default `127.0.0.1:9090`; empty disables). `publicPath: true` also serves
authenticated `GET /v1/metrics`.

Developer MCP over stdio:

```bash
./bin/labmail mcp-stdio --config testdata/config/valid/defaults.yaml --token-file /path/to/token
```

`--token-file` is required unless `auth.mode` is `dev-loopback-unauth`.

Production UI assets: `make web-build` (Node **22.14.0**) copies `web/dist`
into `internal/web/dist`.

### 4. Deliver a message and look at it

```bash
python3 - <<'PY'
import smtplib
from email.message import EmailMessage
m = EmailMessage()
m["From"] = "sut@lab.test"
m["To"] = "inbox@lab.test"
m["Subject"] = "labmail smoke"
m.set_content("hello from the system under test")
with smtplib.SMTP("127.0.0.1", 1025) as s:
    s.send_message(m)
PY

curl -sS http://127.0.0.1:1080/v1/messages
# or open http://127.0.0.1:1080/
```

---

## State loading APIs

Desired state is YAML. The inbox is not. Config revision is a SHA-256 of the
canonical spec (secret *paths*, never values). The store has its own monotonic
`storeGeneration`. Reset re-reads the bootstrap file **and** wipes mail — it
never writes the file.

Every state capability exists on REST and MCP.

| Capability | REST | MCP |
|---|---|---|
| Read redacted spec + revisions | `GET /v1/state` | `mail_state_get` · `labmail://state` |
| Validate a candidate document | `POST /v1/state:validate` | `mail_state_validate` |
| Export canonical YAML + drift | `GET /v1/state:export` | `mail_state_export` |
| Reload bootstrap and wipe inbox | `POST /v1/state:reset` | `mail_state_reset` |
| Dry-run a mutation | `POST /v1/changes:plan` | `mail_change_plan` |
| Apply a mutation | `POST /v1/changes:apply` | `mail_change_apply` |

Auth: `Authorization: Bearer <token>` (or HTTP Basic when
`mode: bearer_and_basic`). Scope `mail.read` for get; `mail.admin` for
validate / export / reset / plan / apply. Health live/ready stay
unauthenticated.

### Read what is loaded

```bash
curl -sS -H "Authorization: Bearer $LABMAIL_TOKEN" \
  http://127.0.0.1:1080/v1/state
```

Typical shape:

```json
{
  "bootstrapRevision": "sha256:…",
  "runtimeRevision": "sha256:…",
  "generation": 4,
  "storeGeneration": 18,
  "drifted": false,
  "messageCount": 3,
  "storeBytes": 4096,
  "loadedAt": "2026-08-17T00:00:00Z"
}
```

`drifted` is `runtimeRevision != bootstrapRevision`. Inbox contents do **not**
set drift. Export the canonical document with `GET /v1/state:export`.

### Validate a candidate before applying it

```bash
curl -sS -X POST -H "Authorization: Bearer $LABMAIL_TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary @- \
  http://127.0.0.1:1080/v1/state:validate <<'JSON'
{
  "state": {
    "apiVersion": "labmail.dev/v1alpha1",
    "kind": "LabMail",
    "metadata": { "name": "lab-sink" },
    "spec": {
      "store": { "maxMessages": 200, "maxBytes": "64MiB", "fullPolicy": "reject" }
    }
  }
}
JSON
```

A failed validate leaves the running snapshot and inbox untouched
(`validation_failed`).

### Plan and apply a live mutation

Coarse ops only — there is no fine-grained record CRUD. Agents that need a
different sink posture change YAML and reset, or apply one of these:

| `op` | Body | Notes |
|---|---|---|
| `replaceSMTPAuth` | `auth`: `{mode, username, passwordFile}` | File must exist at apply |
| `replaceStoreCaps` | `store`: `{maxMessages, maxBytes, fullPolicy}` | Shrink + `reject` fails unless `force` |
| `replaceHideExtensions` | `hideExtensions`: string[] | |
| `replaceAdmission` | `admission`: admission object | |
| `replaceSMTPBehavior` | `behavior`: delays, drop, close-after-verb, per-verb replies | `{}` clears scripting back to stock SMTP |

```bash
REV=$(curl -sS -H "Authorization: Bearer $LABMAIL_TOKEN" \
  http://127.0.0.1:1080/v1/state | jq -r .runtimeRevision)

curl -sS -X POST -H "Authorization: Bearer $LABMAIL_TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary @- \
  http://127.0.0.1:1080/v1/changes:plan <<JSON
{
  "expectedRevision": "$REV",
  "reason": "tighten store cap",
  "operations": [
    {
      "op": "replaceStoreCaps",
      "store": { "maxMessages": 200, "maxBytes": 67108864, "fullPolicy": "reject" }
    }
  ]
}
JSON

# same body against /v1/changes:apply — expectedRevision is required
```

`:plan` is dry-run (same validate/compile, no swap). `:apply` requires
`expectedRevision`. Idempotency key + identity (`reason` + canonical
operations). `revision_conflict` → 409. Success returns
`{ previousRevision, runtimeRevision, generation, diff }`.

### Reset to the mounted file

```bash
curl -sS -X POST -H "Authorization: Bearer $LABMAIL_TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary '{"reason":"restore bootstrap"}' \
  http://127.0.0.1:1080/v1/state:reset
```

Reset re-reads the bootstrap path, validates, then wipes the inbox and swaps
the snapshot. A failed reset leaves **both** config and mail unchanged.
Restart is equivalent: process memory dies; the spill directory is wiped on
the next start.

Normative rules, reserved keys, SMTP TLS validate, and the startup pipeline:
[docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md).
REST shapes:
[docs/06-rest-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/06-rest-api.md).
MCP twins:
[docs/07-mcp-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/07-mcp-api.md).
Capability table:
[docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md).

---

## Build and test

```bash
make format
make lint
make generate
make verify-generated
make test
make test-race
make test-fuzz-smoke
make test-parity
make test-config-compat
make test-docs
make test-container
make test-changelog
make web-test
make web-build
make build
```

Required CI jobs: format, lint, unit, race, fuzz-smoke, generated-file,
documentation, security-scan, changelog, parity, config-compat, container-test,
web. None are optional. `make test-container` needs Docker. A `v*` tag is
refused unless Release `tag-gate` sees those jobs green on the exact SHA.

---

## Documentation

The numbered pack under `docs/` is the source of truth for behavior. Task
summaries do not override it.

### Start here

| Path | Topic |
|---|---|
| [START-HERE.md](https://github.com/hilather/go-lab-maildev/blob/main/START-HERE.md) | Onboarding |
| [AGENTS.md](https://github.com/hilather/go-lab-maildev/blob/main/AGENTS.md) | Contributor / agent rules |
| [docs/README.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/README.md) | Full catalog |
| [CHANGELOG.md](https://github.com/hilather/go-lab-maildev/blob/main/CHANGELOG.md) | Curated history |

### Architecture

| Path | Topic |
|---|---|
| [docs/01-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/01-architecture.md) | Process and package model |
| [docs/02-smtp-semantics.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/02-smtp-semantics.md) | SMTP command table, limits, AUTH/TLS |
| [docs/03-message-store.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/03-message-store.md) | Caps, wait, wipe, spill |
| [docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md) | YAML, revisions, reset, plan/apply |
| [docs/05-control-plane-and-parity.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/05-control-plane-and-parity.md) | Shared capability registry |
| [docs/08-security-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/08-security-architecture.md) | Auth, XSS, receive-only |
| [docs/10-testing-strategy.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/10-testing-strategy.md) | Test layers |

### Interfaces and operations

| Path | Topic |
|---|---|
| [docs/06-rest-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/06-rest-api.md) | Native `/v1` |
| [docs/07-mcp-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/07-mcp-api.md) | MCP tools and resources |
| [docs/09-observability.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/09-observability.md) | Logs, metrics, probes |
| [docs/11-deployment.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/11-deployment.md) | Image, compose, CLI |
| [docs/12-maildev-compat.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/12-maildev-compat.md) | `/email` mapping |
| [docs/13-integration-lab-swap.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/13-integration-lab-swap.md) | mcp-integration-lab swap |
| [docs/known-limitations.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/known-limitations.md) | 1.0 residuals (not a public MTA) |
| [docs/releases/v1.0.0-rc.2.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.2.md) | Current candidate notes |
| [docs/releases/v1.0.0-rc.1.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/releases/v1.0.0-rc.1.md) | First candidate notes |

### Architecture decisions

| ADR | Decision |
|---|---|
| [0001](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0001-use-go.md) | Use Go |
| [0002](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0002-in-tree-smtp-receive-only.md) | In-tree SMTP, receive-only |
| [0003](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0003-ephemeral-inbox-and-gitops.md) | Ephemeral inbox and GitOps |
| [0004](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0004-shared-capability-registry.md) | Shared capability registry |
| [0005](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0005-lab-static-bearer-and-basic-compat.md) | Lab static bearer + Basic compat |
| [0006](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0006-pin-mcp-protocol-versions.md) | Pin MCP protocol versions |
| [0007](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0007-compat-email-surface.md) | Compat `/email` surface |

### Task lists

| Path | Topic |
|---|---|
| [tasks/00-program-board.md](https://github.com/hilather/go-lab-maildev/blob/main/tasks/00-program-board.md) | Work packages 1–14 and milestones |
| [tasks/README.md](https://github.com/hilather/go-lab-maildev/blob/main/tasks/README.md) | Task working rules |

---

## License

[Apache License 2.0](https://github.com/hilather/go-lab-maildev/blob/main/LICENSE)
