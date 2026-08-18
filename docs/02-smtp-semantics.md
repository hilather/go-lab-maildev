# SMTP Receive Semantics

Status: Proposed normative behavior
Owners: SMTP, Architecture
Last reviewed: 2026-08-17 (SMTP-001b + STORE-001)
Related ADRs: 0002

Implementation lives in `internal/smtp/codec` (line IO, reply formatting) and `internal/smtp/server` (session, limits, TLS). No third-party SMTP server library. See [docs/adr/0002-in-tree-smtp-receive-only.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/adr/0002-in-tree-smtp-receive-only.md).

This document is the accept/reject table. Do not invent additional commands, replies, or limits without an ADR.

## Greeting and identity

```
220 <hostname> LabMail ready
```

`hostname` comes from `spec.smtp.hostname` (default `labmail.lab`). The banner **must not** contain “maildev”.

## Commands (1.0)

| Command | Supported | Behavior |
|---|---|---|
| `HELO` / `EHLO` | Yes | Required before MAIL. EHLO lists advertised extensions. |
| `MAIL FROM:` | Yes | Accepts `<>` and any path. Optional `SIZE=`, `BODY=`, `SMTPUTF8`. |
| `RCPT TO:` | Yes | Accepts any path. No directory. Not relay. |
| `DATA` | Yes | After ≥1 RCPT. Terminates on `<CRLF>.<CRLF>`. Dot-unstuff. |
| `RSET` | Yes | Clears transaction; session stays up. |
| `NOOP` | Yes | `250 2.0.0 OK` |
| `QUIT` | Yes | `221 2.0.0 Bye`, close. |
| `HELP` | Yes | `214` with command list. No secrets. |
| `VRFY` | Reply only | Always `252 2.5.2 Cannot VRFY user` (do not imply existence). |
| `EXPN` | No | `502 5.5.1 EXPN not implemented` |
| `AUTH` | Optional | `PLAIN` and `LOGIN` when `spec.smtp.auth.mode != none`. |
| `STARTTLS` | Optional | When `spec.smtp.tls.mode` is `starttls`. |
| `BDAT` | No | `502` |
| `ETRN` / `ATRN` / `TURN` | No | `502` |
| Unknown | — | `500 5.5.1 Command unrecognized` |

## Advertised extensions

Default EHLO keywords:

```
SIZE <maxMessageBytes>
8BITMIME
SMTPUTF8
ENHANCEDSTATUSCODES
STARTTLS          # only when tls.mode=starttls
AUTH PLAIN LOGIN  # only when auth.mode=plain_login (and not withheld for required STARTTLS)
```

`PIPELINING` is **not** advertised in 1.0 (`spec.smtp.hideExtensions` may also hide any of the above). `spec.smtp.hideExtensions` is the maildev `--hide-extensions` equivalent.

## Transaction rules

```text
state: greeting -> helloed -> mail -> rcpt+ -> data -> helloed
```

- Commands are case-insensitive; arguments are not silently lowercased except `HELO`/`EHLO` domain for logs.
- `MAIL` without HELO/EHLO → `503 5.5.1`.
- Nested `MAIL` without `RSET` → `503`.
- `RCPT` without `MAIL` → `503`.
- `DATA` with zero recipients → `503`.
- After successful `DATA`, transaction resets to `helloed` (AUTH/TLS persist).
- Empty reverse-path `MAIL FROM:<>` is accepted (bounce capture).
- Recipients are stored as the **envelope** list; header `To`/`Cc`/`Bcc` are parsed separately and may differ.

## Size, line, and recipient limits

| Limit | Default | Exceeded reply |
|---|---|---|
| Message size (`SIZE` + DATA bytes) | 10 MiB | `552 5.3.4 Message too large` (or `552` at MAIL if `SIZE=` declared over cap) |
| Recipients per transaction | 100 | `452 4.5.3 Too many recipients` |
| Store full (`fullPolicy: reject`) | 1000 messages **or** 256 MiB | `452 4.3.1 Insufficient storage` |
| Command line | 512 octets (RFC 5321 §4.5.3.1.4) | `500 5.5.2 Line too long`, close |
| DATA line | 8192 octets (liberal; HTML) | `500`, abort DATA |
| Session lifetime | 10m | close |
| Command idle | 120s | close |
| DATA idle | 180s | `451 4.4.2 Timeout`, abort |
| Concurrent sessions | 256 | refuse `421 4.3.2` |
| Concurrent sessions per source IP | 32 | refuse `421 4.3.2` |
| In-flight DATA transactions | 8 | `421` |
| In-flight DATA reserved bytes | 64 MiB (`maxInFlightDataBytes`) | `452 4.3.1` |

`maxMessageBytes: 0` is **rejected** at config validate (unbounded is not a lab mode). maildev’s `0 disables` is deliberately not copied.

## AUTH (optional)

`spec.smtp.auth.mode`:

| Mode | Behavior |
|---|---|
| `none` (default) | `AUTH` → `502`. Matches current lab profile. |
| `plain_login` | Advertise `AUTH PLAIN LOGIN`. Unauthenticated `MAIL` → `530 5.7.0 Authentication required`. When `tls.required` is also true, withhold AUTH on the cleartext session (see TLS). |

Credentials: `username` + `passwordFile` (required). Compare with constant-time `subtle.ConstantTimeCompare` after length check. No cram-md5. Successful AUTH is session-scoped. The password is never logged.

**AUTH PLAIN** (RFC 4616), either one-step or two-step:

```
C: AUTH PLAIN <base64("\0" + username + "\0" + password)>
S: 235 2.7.0 Authentication successful
```

```
C: AUTH PLAIN
S: 334
C: <same payload>
S: 235 2.7.0 Authentication successful
```

Bad payload → `535 5.7.8 Authentication failed` (same text for unknown user and wrong password).

**AUTH LOGIN** (de-facto, two challenges; PR 3b). Exact 1.0 transcript:

```
C: AUTH LOGIN
S: 334 VXNlcm5hbWU6
C: <base64(username)>
S: 334 UGFzc3dvcmQ6
C: <base64(password)>
S: 235 2.7.0 Authentication successful
```

`334 VXNlcm5hbWU6` is base64(`Username:`). `334 UGFzc3dvcmQ6` is base64(`Password:`). `AUTH LOGIN` with an initial-response argument is treated as the username reply (skip the first 334). Cancel with `*` → `501 5.5.4`. Empty username or password → `535`.

## TLS (optional)

Single schema. `spec.smtp.tls.mode` is the enum; `required` is a bool that applies only to STARTTLS:

| `mode` | `required` | Behavior |
|---|---|---|
| `off` (default) | must be `false` | No `STARTTLS`. Plain SMTP. Lab contract. |
| `starttls` | `false` | Advertise `STARTTLS`. Cleartext MAIL allowed. |
| `starttls` | `true` | Advertise `STARTTLS`. Cleartext MAIL → `530 5.7.0 Must issue a STARTTLS command first`. Do not advertise AUTH on the cleartext session; cleartext `AUTH` → the same `530` (password is never accepted in the clear). After STARTTLS, advertise AUTH if `auth.mode=plain_login`. |
| `implicit` | — | **Rejected at 1.0 validate** (`smtp.tls.mode: implicit is not supported until 1.1; use starttls or a future listeners.smtpImplicit bind`). Must not share `:1025`. |

Validate rules (fail closed):

- `mode` ∈ {`off`, `starttls`, `implicit`}; unknown → `validation_failed`.
- `mode: implicit` → `validation_failed` in 1.0 (no SMTPS listener field).
- `mode: starttls` requires non-empty `certFile` and `keyFile` that resolve at load.
- `required: true` is illegal unless `mode: starttls`.
- `mode: off` with non-empty `certFile`/`keyFile` → `validation_failed` (unused secrets are errors).

TLS 1.2+ (prefer 1.3). No client-cert SMTP AUTH in 1.0.

## What is accepted vs rejected (normative)

**Accept (250 after DATA) when:**

- Session is helloed (and AUTH/TLS constraints satisfied).
- Envelope has ≥1 RCPT.
- DATA size ≤ `maxMessageBytes`.
- Store accepts the insert.
- MIME parse produces a `model.Message`. A malformed MIME body is still **accepted**: raw bytes are stored, `parseWarning` is set, `text`/`html` may be empty. Labs must be able to inspect broken mail.

**Reject before store:**

- Limit table above.
- TLS/AUTH policy violations.
- DATA without terminator before timeout / disconnect → no store, `421`/`451`.

**Never do:**

- Resolve MX or A/AAAA of the recipient domain.
- Open a TCP connection to deliver.
- Rewrite recipients.
- Generate a DSN or bounce to MAIL FROM.
- Treat `VRFY` as a mailbox probe.

Successful queue reply:

```
250 2.0.0 Queued as <id>
```

`<id>` is the public message id (see [docs/03-message-store.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/03-message-store.md)).

## Live sessions vs config apply

- Every `MAIL`, `RCPT`, and `DATA` re-loads the current config snapshot (atomic pointer). AUTH/TLS/SIZE/hide-extensions/admission follow the new snapshot. A session that is `helloed` when `smtp.auth.mode` flips to `plain_login` gets `530` on the next `MAIL` unless it AUTH’d.
- If `smtp.tls.required` becomes true under a cleartext session, next `MAIL` is `530`.
- STARTTLS-completed sessions keep their TLS state; they do not re-handshake.

## Store epoch interaction

A session captures `epoch` at DATA start (when the reservation is taken). `Insert` with a stale epoch is discarded (`store.ErrStaleEpoch`); SMTP returns `451 4.3.2 Requested action aborted` (not `250`). The operator’s empty inbox stays empty.

## Default lab posture

No AUTH, no TLS required, any MAIL FROM / RCPT TO accepted, SIZE advertised. This preserves `labinfo` and smoke `smtp.SendMail(..., nil, ...)`.
