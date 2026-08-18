# Message Store

Status: Proposed normative behavior
Owners: Store, SMTP, Application
Last reviewed: 2026-08-18 (List/Get/Wait spill consistency)
Related ADRs: 0003

Package `internal/store`. Captured mail is runtime evidence, not desired state. Restart or reset wipes the inbox.

STORE-001 implements `store.Memory` (ULID ids, MIME parse via `internal/mimeparse`, stacked caps, Wait, Wipe epoch, optional spill). SMTP `Insert` takes the epoch captured at DATA start; a mismatch under the insert lock returns `store.ErrStaleEpoch` (`451 4.3.2`). `fullPolicy: reject` returns `store.ErrFull` (`452 4.3.1`). A single message whose resident size exceeds `maxBytes` returns `store.ErrTooLarge` (`552 5.3.4`). `store.Null` remains a discard Sink for tests.

STA-001 adds `ReplaceCaps` / `Configure` / `ResetTo` for live `replaceStoreCaps` and reset. Shrink + `reject` returns `store.ErrOverNewCap` (`store_over_new_cap`) unless `force` or the new policy is `evict_oldest`. Occupancy is judged against the **compiled** candidate spec (last `replaceStoreCaps` wins). Reset preflights store options (including a creatable spill directory) and then `ResetTo` (Wipe + new options under one lock) so a failed reset cannot empty the inbox under the old snapshot. Wipe/`ResetTo` remain the only epoch bumps. SMTP insert stays on the data plane (`store.Sink`), not `app.Service`. AUTH/STARTTLS apply and reset stay reject until SMTP-001b.

Malformed MIME is still stored: raw bytes are kept and `parseWarning` is set.

## Interface

```go
type Store interface {
    Insert(ctx context.Context, epoch uint64, msg *model.Message) (model.InsertResult, error)
    Get(id string, markRead bool) (*model.Message, error)
    List(model.ListQuery) (model.ListResult, error)
    Delete(id string) error
    DeleteAll() (deleted int, err error)
    MarkRead(id string) error
    MarkAllRead() (int, error)
    Wait(ctx context.Context, filter model.MessageFilter) (*model.Message, error)
    Generation() uint64
    Epoch() uint64
    Stats() model.StoreStats
    Wipe() // increment epoch, empty index, unlink spill
    ReplaceCaps(opts Options, force bool) error
    Configure(opts Options) error // occupancy checked against new caps first
}
```

## Addressing and identity

- `id`: Crockford base32 ULID (26 chars) via **`github.com/oklog/ulid/v2`** (MIT; listed under allowed deps). Time-sortable, unique, URL-safe. **Not** maildev’s 8-char `XwgKAxto`. Compat responses still use this id; `/email/:id` accepts it. (8-char ids collide; ULIDs do not.)
- `messageId` (JSON): header `Message-ID` **without** surrounding angle brackets, matching maildev 2.2.1 rest.md (`1412535729…@fbi.gov`). If absent, synthesize `ulid@labmail.lab` (still no brackets on the model field). Raw bytes are never rewritten.
- Envelope vs header addresses are both retained.

## Caps, byte accounting, and eviction

```yaml
store:
  maxMessages: 1000
  maxBytes: 256MiB          # stored resident only (raw + decoded); not in-flight
  fullPolicy: reject        # or evict_oldest
  spillDirectory: ""        # empty = all in RAM; tmpfs still counts
  spillThreshold: 256KiB
```

Caps are **stacked**, not one combined envelope:

```
resident          = Σ (len(raw) + len(text) + len(html) + Σ len(decoded attachment))
reservedInFlight  = Σ (DATA reservations; each ≤ maxMessageBytes)
storeOK           ⇔ (resident + candidate) ≤ maxBytes
                    ∧ messageCount < maxMessages
inFlightOK        ⇔ reservedInFlight ≤ maxInFlightDataBytes
insertAllowed     ⇔ storeOK ∧ inFlightOK
```

- On DATA start, reserve `min(declared SIZE, maxMessageBytes)` (or `maxMessageBytes` if SIZE omitted). That reservation counts **only** toward `smtp.admission.maxInFlightDataBytes` (default 64 MiB), not toward `store.maxBytes`. Over the in-flight cap → do not start DATA (`452 4.3.1`).
- On 250, release the reservation and charge `candidate` to `resident` (must still satisfy `storeOK`; else `452` and the message is discarded). On abort/timeout, release the reservation; nothing is stored.
- `reject`: `Insert` returns `store.ErrFull`; SMTP maps to `452 4.3.1`.
- `evict_oldest`: delete oldest by `receivedAt` until the new message fits; emit `labmail_store_evictions_total`. If a single message’s resident size exceeds `maxBytes`, reject (`552`) — do not evict the whole inbox.
- Spill writes raw (and optionally decoded blobs) under tmpfs. **tmpfs is still RAM.** Spill does not increase the budget; it only bounds Go heap vs kernel page cache. `Wipe` / process exit unlinks files.
- Spill writes use temp names and are committed (rename) only after the insert is accepted. A spill write failure leaves the inbox unchanged (no evict, no generation bump). `Get` / `Wait` / `List` return `store.ErrSpill` if a recorded spill file cannot be read. Startup `New` fails if the configured spill directory cannot be cleared.
- Startup `Wipe`s the configured spill path. Spill is not a mail-directory across restarts.

Default worst-case RSS: stored `maxBytes` (256 MiB) + in-flight `maxInFlightDataBytes` (64 MiB) + ~64 MiB process/heap slack ≈ **384 MiB**. In-flight does **not** shrink inbox capacity.

## Concurrency, epoch, and wait

- Single mutex for metadata index; body blobs are immutable after insert.
- `Wait` uses `sync.Cond` (or a buffered subscriber list) woken on insert.
- `Wait` timeout is the request context. Config cap `store.maxWait: 60s`. Default request timeout `10s`.
- SMTP insert and REST delete may race: `Delete` of an unknown id is `not_found`; a wait that loses the race with `DeleteAll` returns `canceled` / timeout, not a deleted message.

**Store epoch** (distinct from `storeGeneration`):

- `Wipe` (reset, shutdown) increments `epoch` then empties the index.
- A session captures `epoch` at DATA start (when the reservation is taken). `Insert` with a stale epoch is discarded (`store.ErrStaleEpoch`); SMTP returns `451 4.3.2 Requested action aborted` (not `250`). The operator’s empty inbox stays empty.
- `storeGeneration` increments only on **membership/bytes**: insert, delete, wipe, successful evict. **Not** on mark-read / read-all.

## `replaceStoreCaps` that shrinks below current occupancy

| `fullPolicy` after apply | Rule |
|---|---|
| `reject` | Apply **fails** `store_over_new_cap` (HTTP 400) unless the request sets `force: true`, which evicts oldest until under the new caps. |
| `evict_oldest` | Apply succeeds and immediately evicts oldest until under the new caps. |

## Attachments

Stored as child objects of the message:

```go
type Attachment struct {
    ID          string // "<messageULID>:<index>"
    Filename    string // sanitized for download (see security)
    ContentType string
    ContentID   string // used to inline cid: as data: in preview HTML
    Disposition string // inline | attachment
    Size        int
    Checksum    string // sha256 hex of decoded bytes
}
```

Decoded bytes live next to the raw message (RAM or spill). Download never uses the client-supplied filename as a filesystem path.

## Extract regexes

Frozen extract regexes (RE2; implement exactly; do not invent ML). Used by `POST /v1/messages/{id}:extract` / `mail_message_extract`:

```
urlPlain   = (?i)\bhttps?://[^\s"'<>]+
urlAttr    = (?i)(?:href|src)\s*=\s*["'](https?://[^"']+)["']
otpNear    = (?i)(?:code|otp|pin|verify|token)[^\n]{0,40}\b(\d{4,8})\b
otpQuery   = (?i)(?:[?&](?:token|code)=)([A-Za-z0-9_-]{4,64})
```

`urlPlain` runs on `text`. `urlAttr` runs on `html`. Dedup URLs preserving first-seen order. `otpNear` and `otpQuery` run on both bodies; `kind` is `otp_digits` or `otp_query`. `context` is the matching line truncated to 120 runes. Cap 32 URLs and 16 tokens.

## Related documents

- SMTP mapping of store errors: [docs/02-smtp-semantics.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/02-smtp-semantics.md)
- Reset / wipe: [docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md)
- REST wait/extract: [docs/06-rest-api.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/06-rest-api.md)
