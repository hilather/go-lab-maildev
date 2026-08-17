# Wave 2 — Ingest path

Status: not-started
Dependencies: wave 1 (`model`, `config`, `Inbox` interface)
Parallel lanes: W2-STORE, W2-MIME, W2-SAN, W2-SMTP, W2-RELAY

Read: [02-smtp-semantics.md](../02-smtp-semantics.md), [03-mail-model.md](../03-mail-model.md), [ADR 0008](../adr/0008-ephemeral-captured-mail.md), [ADR 0009](../adr/0009-native-go-smtp.md)

Define in W1-MODEL or a tiny W2-IFACE task if missing:

```text
type Ingester interface { Insert(context.Context, ParsedMail) (Email, error) }
type Parser interface { Parse(raw []byte) (ParsedMail, error) }
```

## W2-STORE — Bounded inbox

Exclusive: `internal/store/**`

### Goal

In-memory `Inbox` with mutex, injectable clock/id. List/filter/skip. Delete one/many/all. Mark read / all. Stats. Reset. Subscribe events. Directory backend can be stub returning `unimplemented` until a follow-up **W2-STORE-DIR** (same wave, after memory).

### Required tests

- [ ] Race test: concurrent insert/list/delete
- [ ] Cap: insert at maxMessages returns error used for SMTP 452 (no eviction)
- [ ] Reset empties
- [ ] Get does not mark read (app will)

### W2-STORE-DIR (optional parallel after memory)

Exclusive: `internal/store/directory.go` only
Write `.eml` + attachments; reload; reset deletes files. Tests on `t.TempDir()`.

---

## W2-MIME — Parse RFC 5322

Exclusive: `internal/mime/**`, `testdata/eml/**`

### Goal

`Parse([]byte) (ParsedMail, error)` producing model fields, attachments bytes, calculated BCC, parseWarnings on malformed. Charset decode to UTF-8.

### Required tests

- [ ] Plain text
- [ ] Multipart mixed + attachment
- [ ] Multipart related + CID
- [ ] UTF-8 encoded-word subject
- [ ] 8-bit body
- [ ] Malformed: still returns raw + warnings, no panic
- [ ] Fuzz smoke entry `FuzzParse`

Do not import `smtpd` or `control`.

---

## W2-SAN — HTML sanitizer

Exclusive: `internal/sanitize/**`, `testdata/html/**`

### Goal

Policy that strips `script`, `javascript:` URLs, event handlers. Allows typical email tags. CID rewrite helper `RewriteCID(html, emailID, attachments, baseURL)`.

### Required tests

- [ ] XSS fixtures fail closed (script gone)
- [ ] `cid:` rewritten to attachment URL
- [ ] Empty/nil HTML

---

## W2-SMTP — Receive-only listener

Dependencies: W2-STORE interface, W2-MIME interface (can fake parse in unit tests)
Exclusive: `internal/smtpd/**`

### Goal

`go-smtp` adapter: listen `:0` in tests, accept DATA, enforce SIZE, AUTH, hide-extensions, connection cap. Calls ingest. 250 includes id. No client send API.

### Required tests

- [ ] net.Dial SMTP submit, message in fake inbox
- [ ] AUTH required mode
- [ ] Bad AUTH
- [ ] SIZE exceed → 552, store unchanged
- [ ] Inbox full → 452
- [ ] hide-extensions STARTTLS: not in EHLO
- [ ] Production `internal/smtpd` does not send mail (relay is `internal/relay`)

---

## W2-RELAY — MailDev outgoing client

Exclusive: `internal/relay/**`

### Goal

SMTP client: submit a stored raw message to `outgoing.host:port` with optional AUTH/TLS. Auto-relay rules (allow/deny wildcards, default recipient) as MailDev. No-op interface when host empty. Tests use a local `smtpd` sink or net.Pipe fake.

### Required tests

- [ ] Deliver to test sink; bytes contain original subject
- [ ] `relayTo` override changes RCPT
- [ ] Auto-relay allow/deny fixtures
- [ ] Disabled outgoing returns error used by REST 500
- [ ] Secrets not logged

---

## Wave 2 definition of done

A `_test` in smtpd or a small `internal/ingesttest` sends a message through real SMTP + real parse + real store (integration, still in-process). No HTTP yet.

Mark M2 on the program board.
