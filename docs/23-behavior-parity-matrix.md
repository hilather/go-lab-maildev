# Behavior parity matrix

Status: Proposed normative — **source of truth for “is it ported?”**
Last reviewed: 2026-08-17
Harness: [22-comparison-lab.md](22-comparison-lab.md)

Every row is a MailDev behavior that LabMail must implement. Status values: `not-started` | `oracle-characterized` | `pass` | `n/a`.

`n/a` needs a reason in the notes column. GA forbids `not-started` on required rows.

Oracles: **v2** = MailDev 2.2.1, **v3** = MailDev 3.0 pinned SHA, **lm** = LabMail.

UI: **v2-ui** Angular, **v3-ui** React (LabMail uses v3-ui selectors).

---

## SMTP ingest

| ID | Behavior | REST | UI | v2 | v3 | lm | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| S1 | Accept anonymous SMTP, store message | list contains subject + message-id | inbox shows subject | req | req | req | Fixture `plain.eml` |
| S2 | Multiple RCPT TO | envelope.to length | viewer To: line | req | req | req | |
| S3 | UTF-8 subject/body (SMTPUTF8) | decoded subject/text | visible glyphs | req | req | req | |
| S4 | 8BITMIME body | text round-trip | viewer text tab | req | req | req | |
| S5 | Attachment + filename | attachment GET bytes match | download attachment | req | req | req | |
| S6 | HTML + CID image | html contains rewritten img; attachment GET | html tab shows image | req | req | req | |
| S7 | SMTP AUTH success/fail | fail: no new mail; success: stored | n/a | req | req | req | `auth` profile |
| S8 | hide-extensions STARTTLS | EHLO probe (not REST) | n/a | req | req | req | Protocol test in harness |
| S9 | SIZE / oversized DATA | not stored; list unchanged | inbox unchanged | req | req | req | |
| S10 | Envelope from/to vs headers; calculated BCC | JSON fields | viewer | req | req | req | |

---

## REST inspection (v2 `/email`, v3 `/api/email`; LabMail both)

| ID | Behavior | REST assert | UI assert | v2 | v3 | lm |
| --- | --- | --- | --- | --- | --- | --- |
| R1 | GET list | array; skip pagination | list length | req | req | req |
| R2 | Dotted filter `from.address` | only matches | search/filter if present | req | req | req |
| R3 | GET by id | 404 unknown; 200 body | open row | req | req | req |
| R4 | GET by id marks read | `read: true` after GET; list shows read | unread badge / styling | req | req | req |
| R5 | DELETE one | 200 `true`; gone from list | delete current | req | req | req |
| R6 | DELETE all | empty list | delete all control | req | req | req |
| R7 | PATCH read-all | count; all read | mark all read (v3/lm; v2 if present) | req | req | req |
| R8 | POST bulk delete | v3+lm required; v2 `n/a` (no route) | v3 command palette if any | n/a | req | req |
| R9 | GET html | text/html; no script from fixture XSS | html tab | req | req | req |
| R10 | GET source | RFC 5322 bytes contain fixture token | source tab | req | req | req |
| R11 | GET download | `.eml` disposition; bytes ≈ source | download button | req | req | req |
| R12 | GET attachment | content-type + bytes | attachment link | req | req | req |
| R13 | GET config | `smtpPort`; `isOutgoingEnabled` matches profile | settings if shown | req | req | req |
| R14 | GET healthz | 200 / `true` | n/a | req | req | req |
| R15 | GET reloadMailsFromDirectory | 200; directory profile only | n/a or refresh | req | req | req |
| R16 | Dual prefix | lm only: `/email` ≡ `/api/email` | n/a | n/a | n/a | req |
| R17 | Unauth 401 when basic on | 401 `/email` | browser prompt / blocked | req | req | req |

---

## Relay and auto-relay (profile `relay`; outgoing → `relay-sink`)

| ID | Behavior | REST assert | UI assert | v2 | v3 | lm |
| --- | --- | --- | --- | --- | --- | --- |
| Y1 | Config `isOutgoingEnabled` true | config JSON | Relay control **visible** | req | req | req |
| Y2 | POST `/email/:id/relay` | 200 `true`; **relay-sink** list has same subject/message-id | click Relay | req | req | req |
| Y3 | POST `/email/:id/relay/:relayTo` | sink To: is override address | relay-to if UI supports (v3) | req | req | req |
| Y4 | Relay when outgoing off | 500 MailDev-style error; sink empty | Relay hidden or errors | req | req | req |
| Y5 | Auto-relay all | SMTP to SUT → sink receives without REST relay | n/a (config) | req | req | req |
| Y6 | Auto-relay rules allow/deny | allow fixture relayed; deny not | n/a | req | req | req |
| Y7 | Auto-relay default recipient | sink To: is override | n/a | req | req | req |

Outgoing host in compose **must** be `relay-sink`. MCP: `mail_email_relay` with optional `relayTo` on LabMail ([05-control-plane-and-parity.md](05-control-plane-and-parity.md)); not compared to MailDev MCP.

---

## Real-time UI

| ID | Behavior | REST | UI | v2 | v3 | lm |
| --- | --- | --- | --- | --- | --- | --- |
| L1 | New mail appears without reload | n/a | `waitForSubject` after SMTP | req | req | req |
| L2 | Delete via REST removes row | DELETE then UI | row gone | req | req | req |

Transport may be Socket.IO (oracle) vs `/ws` (LabMail). Assert **UX**, not frames.

---

## UI-only MailDev 3 workflows (LabMail must match v3)

| ID | Behavior | v3-ui + lm | v2-ui |
| --- | --- | --- | --- |
| U1 | HTML / text / headers / source tabs | req | req (equivalent tabs) |
| U2 | Viewport sizes for HTML preview | req | req if present |
| U3 | Search box filters list | req | req if present |
| U4 | Keyboard j/k or equivalent navigation | req | n/a if absent |
| U5 | Command palette actions that map to REST | req | n/a |
| U6 | Dark mode toggle persists in that browser | req | n/a |
| U7 | Favicon/unread badge | req | n/a if absent |
| U8 | Relay + custom recipient | req | Relay without custom if v2 lacks it |

v2-ui rows that are `n/a` still require v3+lm `pass`.

---

## Process / CLI flags (MailDev 2.2.1 `lib/options.js` + v3-compatible flags)

Every MailDev process flag that changes observable behavior has a row. Log-only flags still need a config-compat unit test (wave 1) even if the comparison lab marks UI `n/a`.

| ID | Behavior | REST / protocol assert | UI assert | v2 | v3 | lm | Overlay |
| --- | --- | --- | --- | --- | --- | --- | --- |
| P1 | Default `--smtp 1025 --web 1080 --ip ::` / `0.0.0.0` | SMTP+HTTP reachable on published ports | inbox loads | req | req | req | base |
| P2 | `--base-pathname /maildev` | list at `/maildev/email` (v3: `/maildev/api/email`); unprefixed 404 | UI at `/maildev/` | req | req | req | `compose.basepath.yaml` |
| P3 | `--disable-web` | HTTP not serving UI/REST; SMTP still accepts | n/a | req | req | req | `compose.disableweb.yaml` |
| P4 | `--mail-directory` + `GET reloadMailsFromDirectory` | restart or reload restores `.eml` | refresh shows mail | req | req | req | `compose.directory.yaml` |
| P5 | `--https` + cert/key | `GET` health/list over TLS (insecure skip-verify in harness) | UI over https | req | req | req | `compose.https.yaml` |
| P6 | `--incoming-secure` + cert/key | SMTPS submit succeeds; plain 1025 fails or is not the TLS port | n/a | req | req | req | `compose.smtps.yaml` |
| P7 | `--web-user/--web-pass` | 401 without creds; 200 with | browser basic prompt | req | req | req | `compose.auth.yaml` (same as R17) |
| P8 | `--incoming-user/--incoming-pass` | AUTH success stores; AUTH fail does not | n/a | req | req | req | `compose.auth.yaml` (same as S7) |
| P9 | `--hide-extensions` | EHLO omits named extensions | n/a | req | req | req | flag on oracle (same as S8) |
| P10 | `--web-ip` vs `--ip` | HTTP bind independent of SMTP bind | n/a | req | req | req | unit + compose bind probe |
| P11 | `--verbose` / `--silent` / `--log-mail-contents` | process accepts mail; logs redacted | n/a | req | req | req | config-compat (not live UI) |
| P12 | `--open` (desktop) | n/a — do not implement | n/a | n/a | n/a | n/a | desktop-only |
| P13 | `--mcp` (v3) | LabMail `/mcp` always on by default; v3 optional | n/a | n/a | char | req | LabMail MCP vs REST, not vs MailDev MCP |

`char` = characterize MailDev 3 during CMP-ORACLE; LabMail still required.

---

## Explicitly not compared (still may exist on LabMail)

| Item | Reason |
| --- | --- |
| Node `require('maildev')` | Not a process feature |
| `maildev init` wizard | YAML/flags instead |
| MailDev 3 MCP tool names | LabMail `mail_*`; MCP tested vs REST, not vs MailDev MCP |
| Socket.IO wire protocol | UX compared in L1/L2 |
| `.maildevrc.json` / `maildev.config.ts` | LabMail uses YAML + flags + `MAILDEV_*` ([ADR 0010](adr/0010-yaml-plus-maildev-flags.md)) |
| npm/Homebrew packaging | Out of scope |
| Plugin SDK | Out of scope |

---

## Tracking

Implementation waves update this table’s mental status in the wave handoff. A checked-in `test/parity-lab/STATUS.md` generated from test results is optional; CI failing the case is the real tracker.

GA: all `req` cells `pass`.
