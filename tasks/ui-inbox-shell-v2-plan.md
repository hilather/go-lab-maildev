# Plan: LabMail inbox SPA dark split-pane (after-v2)

Status: Proposed
Last reviewed: 2026-08-29
Owners: UI
Related: UI-001 follow-on; docs/01, 05, 06, 07, 08, 10; ADR 0004; approved mock `mail-redesign/after.png`

This is the implementation plan for the Matt-approved **dark split-pane v2** inbox (not the earlier paper/forest mock). Scope this round is the **shell + Inbox** (list + selected message on `/`). Status / Audit / Reset internals stay; they inherit the new chrome only. Receive-only: no Compose / Relay / Send.

Agent-skills clone/auth was unavailable (`origin auth status` = not logged in; `~/git/agent-skills` missing). Review templates from the task prompt are used as written.

## 0. Investigation (verified against this tree)

### Current UI (matches `before.png`)

- Light theme + full-width green topbar, underlined white nav: `web/src/styles.css` (`--accent: #1f4b3a`), `web/src/App.tsx` `Shell`.
- Inbox is list-only: `web/src/pages/InboxPage.tsx` links to `/messages/:id`. ISO `receivedAt` in `<time>`.
- Clear inbox uses `window.confirm`. Filter is server `subjectContains` only.
- Message page is a separate route: `web/src/pages/MessagePage.tsx`. Tabs include Attachments. Default tab is **text**. HTML uses empty `sandbox` + `/preview`. Delete uses `window.confirm`.
- No InboxPage tests. MessagePage test asserts **no** `markRead=true` on GET.

### Auth / live / preview (keep)

- Session: `POST /v1/session` → HttpOnly `labmail_session`; CSRF in memory only (`web/src/api/client.ts`, `web/src/api/storage.ts`). `apiFetch` sends `X-LabMail-CSRF` on non-GET/HEAD.
- Live: `useInboxLive` — SSE `/v1/events/stream` + 15s watchdog + exclusive 3s poll (`POLL_INTERVAL_MS = 3000`).
- Preview: `PREVIEW_SANDBOX = ""`; `cid:` → `data:` is server-side; remote `http(s)` images stay broken (docs/08).

### Mark-read (the only backend question)

- Display GET already defaults `markRead=false` (`getMessage` comment in `web/src/api/client.ts:171-173`).
- The only **single-message** mark-read API today is `GET /v1/messages/{id}?markRead=true` (`internal/control/rest/messages.go:118-137`). Catalog row `messages.get` is **non-mutating**, scope `mail.read` (`internal/capabilities/catalog.go:147-155`).
- Cookie CSRF is applied only when `auth.UnsafeMethod` is true: POST/PUT/PATCH/DELETE (`internal/auth/session.go:277-285`, `internal/control/rest/auth.go:87-99`). **GET never requires or sends CSRF.**
- Bulk CSRF mutation already exists: `POST /v1/messages:read-all` / `messages.read_all` / `mail_messages_read_all` (`mail.write`). It marks **every** message (`handleReadAll` → `MarkAllRead`). Using it on select would be wrong.
- Domain already has `app.MarkRead` / `store.MarkRead` (no generation bump, no audit — same as `MarkAllRead`). There is **no** REST/MCP binding.
- docs/05 scope table already says `mail.write` includes “mark-read”.
- AGENTS.md: do not invent paths or capability IDs without an ADR. docs/05: MCP `mail_*` names are frozen; add/rename needs ADR + catalog + generate.
- Request: prefer SPA-only **unless impossible without**. CSRF-protecting a single-message mark-read is **impossible** with existing endpoints (GET is not UnsafeMethod; `:read-all` is all-or-nothing).

**Decision:** this change includes the smallest backend addition allowed by “unless impossible without”:

- ADR **0009** (D19): add capability `messages.read`.
- REST `POST /v1/messages/{id}:read` (colon-action, same mux suffix pattern as `{id}:extract` in `internal/control/rest/mux.go:40-59`).
- MCP `mail_message_read` (parity required).
- Scope `mail.write`, `Mutating: true`, `Idempotent: true`, service method `MarkRead`.
- Does **not** bump `storeGeneration` (existing store behavior).
- REST: empty-body POST OK (`optionalBody` alongside `MessagesReadAll`). Response **204** (mirror `messages.delete`). MCP structured `{ok: true}` (mirror `mail_message_delete`).
- Keep `GET ?markRead=true` for programmatic/MCP `mail_message_get` — **SPA must not use it** for display or for select.
- Catalog insert **after** `messages.read_all`, **before** `messages.wait` (docs/05 table order). `TableRowCount` 30 → 31 (`internal/capabilities/registry.go:6`). Update `TestFrozenIDsStable`.
- No new store semantics. No audit (mirror `MarkAllRead`, not `DeleteMessage`).
- Read-only operators (`mail.read` only): select still loads the inspector via GET; mark-read POST is skipped (403 would be wrong to surface as a hard failure).

Do **not** invent other endpoints. Do not touch other hilather repos.

## 1. Visual / shell contract (match `after.png`)

Tokens (exact):

| Token | Value |
|---|---|
| bg | `#0b0c0e` |
| elevated (rail / header) | `#121317` |
| panel (list) | `#181a1f` |
| fg | `#ecece8` |
| muted | `#9a9b97` |
| accent | `#4aa384` |
| danger | `#c45c5c` |

Typography: IBM Plex Sans + IBM Plex Mono. **No new npm dependency.** Load via `index.html` stylesheets to `fonts.googleapis.com` / `fonts.gstatic.com` (IBM Plex Sans 400/500/600/700, IBM Plex Mono 400/500) with system-ui fallbacks if the network is blocked. Do not add `@fontsource/*`.

Layout:

1. **Header 56px:** green status dot + LabMail wordmark; chips `live` (SSE) or `poll` (3s fallback); static `receive-only` chip; **Sign out**. No full-width green bar. No underlined white nav.
2. **Left rail:** Inbox (unread badge = `GET /v1/status` `store.unreadCount`, not the filtered list length), Status, Audit (if `mail.audit.read`), Reset (if `mail.admin`). Active item: tinted background + accent bar on the left edge. No Compose/Relay/Send.
3. **Inbox `/`:** three panes. Middle ~340px “CAPTURED” list: rounded search “Filter subject or from” that filters **as the operator types** (no separate Filter submit button — after.png has none). Drop the legacy “Live update / Store generation / Showing N” ISO chrome. Unread = accent dot + heavier sender weight; relative time; attachment hint (`N attachment(s)`); selected row tinted.
4. **Right inspector:** subject; `from → to · relative time`; Delete (danger outline, write-scope); tabs **HTML / Text / Raw / Headers** (HTML default when `hasHTML`, else Text). HTML body in a light rounded card wrapping the existing empty-sandbox iframe. One-line sandbox note: `sandbox empty · img-src data: only · no remote pixels`. Attachments stay downloadable as a compact list under the tabs (not a fifth tab — mock has four).
5. Other pages: same header + rail; existing page internals restyled only by global dark tokens (no Status/Audit/Reset behavior changes). Reset still requires phrase `RESET` + checkbox (`web/src/ui/forbidden.ts`).
6. Login inherits header (no rail items). Same session form.

## 2. Inbox behavior (SPA)

### Routing

- `/` is the split inbox. Selection is `?id=` via `useSearchParams` so filter state is not lost on select (sibling remount of `/` vs `/messages/:id` would drop it).
- Keep `/messages/:id` as `<Navigate to={`/?id=…`} replace />` so existing links/tests can be updated to the pane, and old URLs still open the inspector.
- `MessagePage` becomes the inspector pane (`MessagePane` export; keep filename or re-export to limit `security.test.ts` churn). Used only as a child of Inbox; no independent “back to inbox” chrome in the split.

### List + filter

- Keep `listAllMessages()` (cursor walk, limit 200). **Do not** send server `from=` — store `from` is **exact address** (`internal/store/memory.go:1020-1027`, `hasAddress`), not contains. Filter is **client-side** case-insensitive substring on subject, from display name, from address, and `envelope.from`.
- Live inbox stays SSE + 15s watchdog + exclusive 3s poll. **One** `EventSource` only: a small provider (Shell or `InboxLiveProvider`) owns `useInboxLive` and fans out `onChange`. Do not mount `useInboxLive` in both Shell and InboxPage (that would open two streams).
- List refresh must ignore stale overlapping responses (existing `refreshSeq` pattern).
- Unread badge updates when status/list refreshes and when a successful mark-read decrements locally. Filter must not hide unread from the badge.
- List sender line is **display name if non-empty, else address** (after.png shows `App`, not `App <noreply@…>`). Inspector metadata still uses `formatAddress` (`Name <addr>`).

### Select → load → mark-read

1. Click/keyboard select sets `?id=`.
2. Inspector `GET /v1/messages/{id}` with **no** `markRead` query (existing `getMessage`).
3. If the list item (or GET body) is unread **and** `mail.write`, call new `markMessageRead(id)` → `POST /v1/messages/{id}:read` with CSRF. On 204, set that item `read: true` locally (do not wait for a generation bump; there is none).
4. Tests must assert: display GET has no `markRead=true`; mark-read is a **POST** with `X-LabMail-CSRF`; selecting does not use `window.confirm`.

### Relative time

- New helper `web/src/ui/relativeTime.ts` (no date-fns). Same local calendar day → `HH:MM` (mock shows `11:28`). Older same year → short date (e.g. `29 Aug`). Else include year. Accept `now` for tests. Keep `dateTime={receivedAt}` on `<time>` for the ISO instant.

### Confirms

- Shared in-page confirm (inline panel, not `window.confirm`, not a modal library).
- Clear inbox: write-scope, **not** `type="submit"` / `.primary`. Confirm then `DELETE /v1/messages`.
- Delete message: danger outline; confirm then `DELETE /v1/messages/{id}`; clear `id` query.

### Empty / error

- No selection: inspector empty state (do **not** auto-select first — that would mark-read on load).
- Missing id: error in inspector, list remains.
- Forbidden labels stay gated by `FORBIDDEN_CONTROL_LABELS`.

## 3. Implementation steps (order)

1. **ADR 0009** `docs/adr/0009-single-message-mark-read.md` (D19). Link from `docs/README.md`. Add path to `scripts/checkdocs` `RequiredRootDocs` and its test if the list is asserted.
2. **Capability + adapters (no new domain logic):**
   - `MessagesRead ID = "messages.read"` in `internal/capabilities/id.go`.
   - Catalog row; `TableRowCount = 31`; `TestFrozenIDsStable` insert after `MessagesReadAll`.
   - REST: `handleMarkRead` in `messages.go`; dispatch in `handlers.go`; `optionalBody`.
   - MCP: `addTool("mail_message_read", desc, true /* mutating */, true /* idempotent */, …)` calling `svc.MarkRead` (`addTool` signature is `mutating, idempotent` at `internal/control/mcp/tools.go:284`).
   - `make generate` (do not hand-edit `api/*.json`).
   - REST contract: POST marks one message; GET default still unread; POST requires CSRF on cookie session; 404 unknown id; 403 without `mail.write`.
   - MCP contract + `make test-parity`.
3. **SPA client:** `markMessageRead` in `web/src/api/client.ts` + client test (CSRF header on POST).
4. **Chrome:** `web/src/styles.css` tokens; `App.tsx` header + rail. A single live provider owns `useInboxLive` + `getStatus` (`store.unreadCount`) so the Inbox badge works on every signed-in page without a second EventSource. Inbox local mark-read decrements the badge immediately; the next status refresh confirms.
5. **Inbox + inspector:** rewrite `InboxPage` + extract pane from `MessagePage`. Confirm UI. Relative time. Filter. Tabs/sandbox note.
6. **Tests (web):** see §4. Update `MessagePage.test.tsx` (tab accessible name becomes `/^HTML$/i`, not `/HTML preview/i`; keep empty-sandbox + no `markRead=true` on GET). Update `security.test.ts` (still forbids `innerHTML` / storage / relaxed sandbox). Add Inbox/Shell tests.
7. **Docs + changelog (same change):**
   - `docs/01-architecture.md` Embedded operator UI table (split inbox, chips, relative time, mark-read POST). Last reviewed.
   - `docs/05` capability table (insert `messages.read` after `messages.read_all`), PARITY_REQUIRED mention, Related ADRs; Last reviewed. `docs/01` Related ADRs line includes 0009.
   - `docs/06` POST `:read`; Last reviewed.
   - `docs/07` tools table; Last reviewed.
   - `docs/08` replace “the SPA does not pass `markRead=true`” with: display GET never marks read; select uses `POST …:read` + CSRF; `GET ?markRead=true` remains API-only. Last reviewed.
   - `docs/10` inbox UI layer mentions shell/unread/select-mark-read/sandbox/relative-time tests.
   - `web/README.md` pages description.
   - `CHANGELOG.md` Unreleased Added/Changed (observable `web/` + `/v1` path).
8. **`make web-build`** copies hashed assets into `internal/web/dist` (required; `spa_test.go` only asserts “LabMail” in HTML).
9. **Completion commands (local, then CI):** `make format`, `web-test`, `web-build`, `test`, `test-parity`, `test-docs`, `test-changelog`, plus `generate`/`verify-generated` after catalog edits. Full required set before merge; do not merge.

## 4. Tests (must exist; bug-fix style where behavior changes)

| Behavior | Where |
|---|---|
| Shell: header chips (`receive-only`, live/poll), rail labels, no Relay/Compose/Send | Extract `Shell` and test under `MemoryRouter` — do not render `App`’s `BrowserRouter` inside `renderApp` |
| Unread badge shows count and drops after successful mark-read | Inbox/Shell test with mocked list + POST |
| Select loads GET **without** `markRead=true`, then POST `:read` with CSRF | `InboxPage.test.tsx` |
| Read-only session: select does not POST `:read` | same |
| Empty sandbox iframe + sandbox note | Message pane test (update existing) |
| Relative time: same-day `HH:MM` vs older date | `relativeTime.test.ts` |
| Clear inbox / delete: in-page confirm, **no** `window.confirm` | Inbox + pane tests |
| Reset still gated on `RESET` + checkbox | existing `ResetPage.test.tsx` |
| CSRF on new POST | `client.test.ts` + REST session test |
| `messages.read` parity / frozen IDs / generate clean | Go catalog + rest + mcp + `make test-parity` |
| Static XSS / no web storage | existing `security.test.ts` |

Do not delete or weaken the existing “GET must not include `markRead=true`” assertion; extend it to “POST `:read` is the select mutation.”

## 5. Non-goals

- Redesign Status / Audit / Reset forms beyond dark tokens + shared shell.
- Compose / Relay / Send / outgoing config.
- New npm/Go dependencies.
- Implicit SMTPS, chaos engine, persistence, other repos.
- Using `GET ?markRead=true` from the SPA.
- Using `POST :read-all` to mark one message.
- Auto-selecting the first message on load.
- `window.confirm`.
- Hand-editing generated OpenAPI/manifest/MCP JSON.

## 6. Risks

- **Capability freeze:** adding a row is a public-surface change; ADR 0009 + generate + docs/05/07 + `TableRowCount` 30→31 + `TestFrozenIDsStable` must land together or `verify-generated` / catalog tests fail. Mutation extras (dry-run, reason, audit) are **not** added; mirror `messages.read_all`, which also omits them.
- **Dual SSE:** forbidden; one live provider only.
- **Google Fonts:** air-gapped operators see system fallback. Acceptable residual; do not block 1.0 on self-hosted woff2 this round.
- **Narrow viewports:** mock is desktop. Stack list above inspector under ~900px so the shell remains usable; not a second visual language.
- **Unread vs filter:** badge must use status/unfiltered count, not the filtered list length.

## 7. Done when

UI matches `after.png` (dark three-pane inbox), invariants in the request hold, tests in §4 exist and pass, PR open, required CI green. **Do not merge.**
