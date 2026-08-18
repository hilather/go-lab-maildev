# State and Configuration

Status: Proposed normative behavior
Owners: Configuration, Application
Last reviewed: 2026-08-17 (CFG-001)
Related ADRs: 0003

Desired state is YAML. The inbox is not. Config revision is a content hash of the canonical spec. Message store has its own monotonic `storeGeneration`. Reset reloads YAML **and** wipes mail. See [docs/adr/0003-ephemeral-inbox-and-gitops.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0003-ephemeral-inbox-and-gitops.md).

## YAML bootstrap schema

One document. UTF-8. No aliases/anchors. No duplicate keys. No multi-doc streams. Max file size 1 MiB. Unknown fields are errors (`yaml.Decoder.KnownFields(true)`). Durations use Go syntax (`30s`, `5m`). Byte sizes use binary units (`10MiB`, `256KiB`) via a typed `config.ByteSize` (bare numbers are rejected). Secret values are **file references**, never inline (except optional dev-only `environment:` when `LABMAIL_ALLOW_ENV_SECRETS=1`, default off in the image).

```yaml
apiVersion: labmail.dev/v1alpha1
kind: LabMail
metadata:
  name: lab-sink
spec:
  listeners:
    smtp:
      address: ":1025"
    management:
      address: ":1080"
      restPath: /v1
      mcpPath: /mcp
      compatEnabled: true          # /email, /healthz, /config
      tls:
        enabled: false
        certFile: ""
        keyFile: ""

  smtp:
    hostname: labmail.lab
    maxMessageBytes: 10MiB         # maildev default is 50MiB; intentional
    maxRecipients: 100
    hideExtensions: []             # e.g. [STARTTLS, SMTPUTF8]
    auth:
      mode: none                   # none | plain_login
      username: ""
      passwordFile: ""
    tls:
      mode: off                    # off | starttls | implicit (implicit rejected in 1.0)
      required: false              # legal only when mode=starttls
      certFile: ""
      keyFile: ""
    admission:
      maxSessions: 256
      maxSessionsPerIP: 32
      maxInFlightData: 8
      maxInFlightDataBytes: 64MiB
      sessionTimeout: 10m
      commandIdle: 120s
      dataIdle: 180s

  store:
    maxMessages: 1000
    maxBytes: 256MiB               # stored resident only (raw + decoded)
    fullPolicy: reject             # reject | evict_oldest
    maxWait: 60s
    spillDirectory: ""
    spillThreshold: 256KiB

  ui:
    enabled: true                  # false hides SPA only; REST/MCP stay up

  management:
    auth:
      mode: bearer_and_basic       # bearer | bearer_and_basic | dev-loopback-unauth
      tokens:
        - id: admin
          secretFile: /run/secrets/labmail-token
          role: administrator
          scopes: [mail.read, mail.write, mail.admin, mail.audit.read]
      basic:
        username: admin
        passwordFile: /run/secrets/maildev-web-password
        tokenRef: admin            # same principal as tokens[id=admin]
    mcp:
      allowLegacyClients: false    # lab overlay: true (D17 / TacLab knob)
    originAllowlist: []            # present non-loopback Origin default-deny
    bodyLimit: 1MiB
    requestsPerSecond: 32
    burst: 64
    maxConcurrent: 256

  observability:
    logLevel: info                 # debug | info | warn | error
    metrics:
      listen: "127.0.0.1:9090"     # empty = disabled; lab overlay may use ":9090"
      publicPath: false            # true → authenticated GET /v1/metrics
    audit:
      ring: 128
```

## Reserved / rejected keys

Fail closed even if someone tries to smuggle them as unknown-looking names after normalize:

```
outgoing, outgoingHost, outgoing-host, outgoingPort, outgoingUser,
outgoingPass, outgoingSecure, autoRelay, auto-relay, autoRelayRules,
auto-relay-rules, relay, smarthost, smartHost, forwardTo, mx, deliver
```

Normalize strips dashes/underscores/case before the reserved-name check so `--auto-relay` / `auto_relay` / `autoRelay` all die the same way. This is the in-process equivalent of `mcp-integration-lab/internal/maildev.relayFlags`.

## SMTP TLS validate rules (fail closed)

Copied here so CFG-001 does not have to re-open [docs/02-smtp-semantics.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/02-smtp-semantics.md):

- `mode` ∈ {`off`, `starttls`, `implicit`}; unknown → `validation_failed`.
- `mode: implicit` → `validation_failed` in 1.0 (`smtp.tls.mode: implicit is not supported until 1.1; use starttls or a future listeners.smtpImplicit bind`).
- `mode: starttls` requires non-empty `certFile` and `keyFile` that resolve at load.
- `required: true` is illegal unless `mode: starttls`.
- `mode: off` with non-empty `certFile`/`keyFile` → `validation_failed` (unused secrets are errors).
- `maxMessageBytes: 0` is **rejected** at config validate (unbounded is not a lab mode).

## Revisions

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

- `bootstrapRevision` / `runtimeRevision`: SHA-256 of canonical normalized spec (secrets as reference paths, never values).
- `generation`: process-local config swap counter.
- `storeGeneration`: increments on insert/delete/wipe/evict only. **Not** on mark-read. Cursors bind this value.
- `drifted`: `runtimeRevision != bootstrapRevision`. Inbox contents do **not** set `drifted`.

Config mutations (plan/apply) use `expectedRevision` = `runtimeRevision`. Inbox mutations use `If-Match` / `expectedStoreGeneration` on the store generation (optional on delete-one; required on `DELETE /v1/messages` wipe when `requireStoreGeneration: true` in the request — default false for compat wipe).

## Reset

`POST /v1/state:reset` / `mail_state_reset`:

1. Re-read bootstrap path (never write it).
2. Validate + compile. On failure, leave current config **and** inbox unchanged; return `validation_failed`.
3. `store.Wipe()` — **the only epoch bump**. Empties the index, unlinks spill, increments `epoch` and `storeGeneration`. In-flight DATA inserts with the old epoch fail `451`.
4. Atomically swap the config snapshot, clear the idempotency LRU, increment config `generation`.
5. Existing SMTP sessions re-load the new snapshot on the next MAIL/RCPT/DATA (or die on QUIT/timeout).
6. Audit `state.reset`.

Restart is equivalent: process memory dies; spill dir is wiped on next start.

## Plan / apply operations

Envelope (LabDNS-shaped):

```json
{
  "expectedRevision": "sha256:…",
  "idempotencyKey": "01J…",
  "reason": "tighten store cap",
  "force": false,
  "operations": [
    {
      "op": "replaceStoreCaps",
      "store": { "maxMessages": 200, "maxBytes": "64MiB", "fullPolicy": "reject" }
    }
  ]
}
```

| `op` | Body fields | Notes |
|---|---|---|
| `replaceSMTPAuth` | `auth`: `{mode, username, passwordFile}` | File must exist at apply |
| `replaceStoreCaps` | `store`: `{maxMessages, maxBytes, fullPolicy}` | Shrink + `reject` fails unless `force` |
| `replaceHideExtensions` | `hideExtensions`: string[] | |
| `replaceAdmission` | `admission`: admission object | |

`:plan` is dry-run (same validate/compile, no swap). `:apply` requires `expectedRevision`. Idempotency: key + identity (`reason` + canonical operations). Failures are not cached. `revision_conflict` → 409. `store_over_new_cap` → 400, `code: store_over_new_cap` (not `validation_failed`). Success returns `{ previousRevision, runtimeRevision, generation, diff }`.

Fine-grained record CRUD is unnecessary. Agents that need a different sink posture should change YAML and reset, or apply one of the above.

Idempotency LRU default 256; reset clears it.

## Startup

```text
read file
 -> reject unknown fields and reserved names
 -> decode versioned schema
 -> normalize names, durations, byte sizes, defaults
 -> validate cross-references and policy constraints
 -> compile snapshot
 -> compute bootstrap and runtime revisions
 -> wipe configured spill path
 -> bind SMTP then management
```

A normal startup does not bind listeners when bootstrap validation or compile fails.

## Compatibility promise

`labmail.dev/v1alpha1` is fail-closed; additive fields only after schema bump or explicit defaulting ADR.
