# Plan: Remaining SPA pages — approved chrome (after-v2)

Status: Proposed
Last reviewed: 2026-08-29
Owners: UI
Related: PR #18 follow-up; docs/01, 08, 10; `mail-redesign/after.png`
Branch: `cursor/labmail-inbox-spa-v2-2737` (existing PR, do not open a new one, do not merge)

Matt approved applying the inbox chrome to **all remaining SPA pages**. Same product, same routes, same behavior. Receive-only. Do not invent a login product.

Agent-skills Origin clone/auth was unavailable (`~/git/agent-skills` missing). Review templates from the task prompt are used as written.

## 0. Investigation (verified against this tree)

### Routes (`web/src/App.tsx:145-166`)

| Path | Component | Signed-in chrome |
|---|---|---|
| `/login` | `LoginPage` | Header only (no rail, no live chips) via `AppShell` unsigned branch |
| `/` | `InboxPage` | Header + rail + split inbox — **already matches after.png; out of scope except shared CSS** |
| `/messages/:id` | `MessageRedirect` → `/?id=` | Not a page |
| `/status` | `StatusPage` | Header + rail |
| `/audit` | `AuditPage` | Header + rail (nav gated by `mail.audit.read`) |
| `/reset` | `ResetPage` | Header + rail (nav gated by `mail.admin`) |
| `*` | `Navigate` to `/` | |

No other page files exist under `web/src/pages/`.

### What is already dark

`:root` in `web/src/styles.css` already has the approved tokens (`--bg #0b0c0e`, `--elevated #121317`, `--panel #181a1f`, `--fg #ecece8`, `--muted #9a9b97`, `--accent #4aa384`, `--danger #c45c5c`). IBM Plex is loaded in `web/index.html`. `--font-sans` lists `"Segoe UI"` only as a **fallback** after IBM Plex (air-gap residual from the inbox plan). There is **no** leftover `#1f4b3a` / paper / navy theme in `web/src/styles.css`.

Remaining pages inherit those tokens but their **interiors are unstyled generic HTML**, not money-view panels:

- `LoginPage.tsx`: `page page--narrow`, bare `<h1>`, `<fieldset>` (UA border), radios, `button type="submit"` (no class).
- `StatusPage.tsx`: `page`, bare `<h1>`/`<h2>`, unstyled `<dl>`, `<ul>`, `<pre class="raw">`. Loading/error are a lone `<p>`.
- `AuditPage.tsx`: `page`, `table.data` (panel bg already, no radius / muted header treatment), empty copy is a bare `<p>`.
- `ResetPage.tsx`: same form stack as login; submit is unclassed `type="submit"`. Phrase `RESET` + checkbox stay (`web/src/ui/forbidden.ts`).

`ConfirmBar` (inbox clear/delete) is already a dark in-page panel. Reset does **not** use it and must not start using `window.confirm`.

### CSS trap (must fix, not just add a class)

```160:168:web/src/styles.css
button[type="submit"],
button.primary {
  background: var(--accent);
  color: #0b0c0e;
  border: 0;
  ...
}
```

`.btn-danger` is a class (`0,1,0`). `button[type="submit"]` is `0,1,1` and **wins**. Putting `class="btn-danger"` on Reset’s submit without changing this rule leaves a filled accent button. Plan: stop treating every submit as primary; require `.primary` on Sign in; Reset uses `.btn-danger` only.

Current `type="submit"` sites: Login Sign in, Reset LabMail. `ConfirmBar` buttons are `type="button"`.

### Invariants that must not change

- Receive-only: no Compose / Relay / Send (`FORBIDDEN_CONTROL_LABELS`).
- Login product: bearer **or** Basic → `POST /v1/session`; token not written to web storage. No SSO, no extra fields.
- Reset: phrase `RESET` + checkbox; `canSubmitReset`; reread bootstrap + wipe.
- SSE + 15s watchdog + exclusive 3s poll via existing `LiveProvider` (one EventSource). Do not mount live on `/login`.
- HTML iframe empty sandbox (`PREVIEW_SANDBOX === ""`).
- GET must not mark read; select still `POST …:read` + CSRF.
- No new backend, capability, npm/Go dep, or route.
- Same PR #18. Hold for Matt. Do not merge.

## 1. Visual contract (interiors)

Match after.png language on remaining pages:

| Surface | Treatment |
|---|---|
| Page frame | Signed-in: existing header + rail. Login: existing header only. |
| Title | Page `<h1>` weight 600, fg; optional muted one-line intro. |
| Card | `--panel` `#181a1f`, 1px `--line`, 8–12px radius, padding (same family as inspector / confirm-bar / captured). |
| Section labels | Small uppercase muted (same as `.captured__title`). Status: Store / Listeners / Revisions. |
| Inputs | Dark field, `--line` border, 6–8px radius, IBM Plex inherit (same as `.captured__search` / `.field input`). |
| Radios / checkbox | Native controls; labels fg; fieldset border reset (no UA gray box). |
| Primary (Sign in) | Filled `--accent` `#4aa384`, fg `#0b0c0e` — **only** `.primary`. |
| Destructive (Reset) | Danger **outline** like Delete (`.btn-danger`), not filled accent. Still `type="submit"`. |
| Tables | Panel card; muted `<th>`; `--line` row borders; mono ids. |
| Empty / loading | Muted copy (`.empty-state`), not a paper well. |
| Error | Existing `.banner-error` (dark red wash). Fix its inbox-only `margin: 0.4rem 0.75rem` so cards are not indented oddly. |
| Success (reset notice) | Muted/status, not a green paper banner. |

Do **not** invent Login marketing, SSO, “welcome”, or extra chips on the unsigned header.

## 2. Implementation steps (order)

1. **CSS primitives** in `web/src/styles.css` (no new CSS file, no new npm):
   - `.panel` card.
   - `.page-title` / `.section-label`.
   - `.kv` definition list (Status store stats).
   - `.empty-state`.
   - `fieldset` / `legend` reset.
   - `button[type="submit"]` **no longer** implies primary. `.primary` = filled accent. `.btn-danger` remains outline. Sign in gets `class="primary"`.
   - `.banner-error` margin: use block spacing, not the captured-list inset.
2. **LoginPage** — wrap existing form in `.panel`. Keep copy, modes, field names, `noValidate`, error `role="alert"`, navigate `/` on success. Add `class="primary"` on Sign in.
3. **StatusPage** — keep `getStatus` fields. Card(s) + `.kv` for store; listeners list; revisions stay `<pre class="raw">`. Loading/error use `.empty-state` / `.banner-error`.
4. **AuditPage** — keep columns Time / Capability / Actor / Result / Message (ISO `ev.time`, not relative — same behavior). Wrap table in `.panel`. Empty: `.empty-state` “No audit events.”
5. **ResetPage** — wrap form in `.panel`. Submit `class="btn-danger"`. Phrase + checkbox + `canSubmitReset` unchanged. Do not switch to `ConfirmBar` or `window.confirm`.
6. **Session loading** (`RequireSession` / `RedirectIfSignedIn` in `App.tsx`) — keep “Checking session…”; apply `.empty-state` so the unsigned/signed gate is not a bare paper paragraph.
7. **Tests** — see §3. Do not delete or weaken Login token-storage, Basic switch, empty-token, Reset gate, Inbox GET-not-mark-read, sandbox, or forbidden-control tests.
8. **Docs + changelog (same change):**
   - `docs/01-architecture.md` Pages row: remaining interiors use the same dark panel language (not inbox-only). Last reviewed.
   - `docs/10-testing-strategy.md` Inbox UI layer: login/status/audit/reset chrome tests. Last reviewed.
   - `web/README.md` one line that remaining pages share the inbox chrome.
   - `CHANGELOG.md` Unreleased Changed (observable `web/` interiors).
9. **`make web-build`** so `internal/web/dist` matches (hashed CSS/JS).
10. Local: `npm --prefix web test`, `make web-test`, `make web-build`, `make test-docs`, `make test-changelog`. Then push to #18 and wait for CI. Do not merge.

No `make generate`. No ADR. No capability change.

## 3. Tests

| Behavior | Where |
|---|---|
| Login still POSTs bearer and stores nothing | existing `LoginPage.test.tsx` (keep) |
| Login chrome: panel + primary Sign in; no Compose | `web/src/pages/chrome.test.tsx` (reads `LoginPage` markup + `styles.css`) |
| Status store labels + panel; loading/error paths | new `StatusPage.test.tsx` (page had none); mock `/v1/session` + `/v1/status` like `shell.test.tsx` |
| Audit table headers + empty state | new `AuditPage.test.tsx`; mock `/v1/audit` empty and one-row |
| Reset still disabled until `RESET` + checkbox | existing `ResetPage.test.tsx` (keep) |
| Reset submit is danger outline class, not `.primary` | extend Reset test |
| CSS has `--accent: #4aa384` and no `#1f4b3a` | `chrome.test.tsx` reads `web/src/styles.css` |
| GET still no `markRead=true`; empty sandbox | existing Inbox/Message/security tests (unchanged) |

Chrome tests assert **observable classes and copy**, not computed colors (jsdom).

## 4. Non-goals

- Inbox / inspector redesign (already shipped).
- New login product (SSO, remember-me, extra identity fields).
- Compose / Relay / Send.
- Relative timestamps on Audit (behavior change).
- New EventSource on `/login`.
- New backend / ADR / deps.
- New PR.
- Self-hosting IBM Plex woff2.

## 5. Risks

- **Submit cascade:** missing the `button[type="submit"]` change leaves Reset green. Step 1 is mandatory.
- **Login tests** render `LoginPage` without `AppShell` — do not require masthead in those tests; add a shell-wrapped case only if asserting header on `/login`.
- **Google Fonts:** air-gapped operators still see system fallback. Keep Segoe as fallback after IBM Plex (same as inbox plan); do not set Segoe as the designed face.
- **Double padding** on login (`stage--solo` + `.page`) is acceptable; do not invent a second layout language.

## 6. Done when

Login / Status / Audit / Reset interiors match the after.png family (dark panels, accent `#4aa384`, IBM Plex, outlined danger, muted empty states). Invariants in §0 hold. Tests in §3 pass. Pushed to PR #18. CI green. **Do not merge. Hold for Matt.**
