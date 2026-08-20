# ADR 0008: Origin-policy escape hatches

Status: Accepted
Date: 2026-08-20
Decisions: D18

## Context

Management HTTP origin is gated by `auth.CheckOrigin` (REST outer gate, then MCP and compat inner gates). A present non-loopback `Origin` is denied unless it is an exact `spec.management.originAllowlist` entry. Missing Origin is allowed. Loopback Origins (`localhost` byte-for-byte, or `net.ParseIP(host).IsLoopback()`) are allowed. `OPTIONS` is `403` `forbidden` / `CORS is disabled`. No CORS headers. `dev-loopback-unauth` skips **auth** on loopback `RemoteAddr` only. MCP Streamable HTTP sets `DisableLocalhostProtection: true`; LabMail `CheckOrigin` is the only MCP origin gate.

The embedded inbox SPA at `GET /` is typically 200 under the empty default because top-level navigation often sends no `Origin`. Hashed Vite assets in `internal/web/dist/index.html` load as module scripts and CSS with `crossorigin`, so the browser **does** send `Origin`. Empty allowlist then 403s non-loopback SPA JS (`forbidden` / `origin is not allowed`). That is correct DNS-rebinding default-deny for production. It is also why a published LAN UI or a remotely hosted management plane cannot load JS/CSS until the operator lists the browser origin.

Exact entries remain the 1.0 answer for a stable Origin. They are painful for ephemeral hostnames (Codespaces, preview URLs) and DHCP LAN IPs. Operators reach for `dev-loopback-unauth` or “disable origin checks,” which either does not help remote-dev or silently weakens production. A CORS success path is not required to load same-origin hashed JS once Origin is allowed.

`docs/04-state-and-configuration.md` Compatibility promise: `labmail.dev/v1alpha1` is fail-closed; **additive fields only after schema bump or explicit defaulting ADR.** Today `originAllowlist` is an existing `[]string` that accepts any string at validate (including `"*"`, which is an exact miss, not a wildcard). Tightening validate and giving two existing strings sentinel meaning needs an explicit ADR.

This ADR records **D18**. It does **not** claim the matcher, fail-closed validate, or live-read wiring is already shipped. Until the implementation change lands, `CheckOrigin` remains exact-match only, `originAllowlist` remains unchecked at validate, and the numbered pack (`docs/01`–`11`, known-limitations, examples) still describes exact-list-only behavior.

## Decision

**D18 — Operator-opt-in origin hatches live only on the existing field `spec.management.originAllowlist`. Production default stays fail-closed. No CORS success path in 1.0.**

### D18a — Same field; no new knob

Hatches live **only** in `spec.management.originAllowlist`. No new YAML field, no `devMode` / `disableOriginCheck` / `allowAllOrigins` boolean. Fail-closed unless set.

### D18b — Sentinel `"*"`

A list entry whose **trimmed** value is exactly `*` allows any **http/https** Origin. It does **not** allow `file://`, the literal header `Origin: null` (parses with empty scheme/host), missing-scheme, or non-http(s). Residual: DNS-rebinding origin defense is off for all http(s) Origins; auth stays on. `"*"` must be quoted in YAML (unquoted `*` is a YAML alias).

### D18c — Sentinel `"private"`

A list entry whose trimmed value EqualFolds `private` allows Origins whose host parses as an IP and `ip.IsPrivate()` (Go `net.IP.IsPrivate`: RFC 1918, RFC 4193 ULA, RFC 6598 CGNAT). Hostnames (`devbox.local`) do **not** match. Loopback remains independently allowed. Link-local (`169.254/16`, `fe80::/10`) does **not** match. IPv4-mapped `::ffff:192.168.0.0/112` **does** match because Go `IP.IsPrivate` uses `To4()`. Classifier is stdlib, not a hand-rolled table. `"private"` is every such Origin host (any port), not “this process’s bind address.”

### D18d — `"*"` is the only wildcard

Any other entry containing `*` (after trim) is `validation_failed`. Matching is exact EqualFold or a sentinel, never glob. Host wildcards (`https://*.github.dev`) are out of 1.0.

### D18e — Sentinels OR exact

Sentinels coexist with exact entries (OR). `"*"` dominates. Duplicates allowed. Order irrelevant. Mixing `"*"` and `"private"` is valid.

### D18f — CORS / OPTIONS frozen off

CORS / OPTIONS-as-success is **out of 1.0**. `OPTIONS` remains `403` `forbidden` / `CORS is disabled` even when `"*"` or `"private"` is set, including loopback. No `Access-Control-Allow-*`. REST’s OPTIONS gate runs before mounts, SPA `tryUI`, and inflight. `OPTIONS /mcp` on the management listener is 403 at the REST outer gate and never reaches MCP’s POST-only 405. Direct MCP handler tests (no REST wrap) still see 405 for OPTIONS after origin passes. Same-origin hashed JS does not need CORS once Origin is allowed. Remote Vite without a proxy is not a 1.0 requirement.

### D18g — Defaults stay empty and authenticated

Default YAML, `examples/labmail.yaml`, and the image/container smoke stay `originAllowlist: []` (or omit → default `[]`) and `auth.mode: bearer_and_basic`. Do not ship `"*"` in the image default.

### D18h — Domain logic in `internal/auth`

`auth.CheckOrigin(origin string, allowlist []string) error` stays the matcher. REST, MCP, and compat are one-line adapters; they do not grow independent origin/CORS code. No new capability ID. Origin is middleware, not a registry row.

### D18i — No live apply op

Allowlist is **not** a `changes.apply` op in 1.0. YAML edit + **reset or restart**. The frozen plan/apply table has no `replaceOriginAllowlist`.

### D18j — `dev-loopback-unauth` is not an origin hatch

`dev-loopback-unauth` remains auth-only on loopback `RemoteAddr`. Origin policy is independent. Do not recommend it for remote-dev (the server `RemoteAddr` is not the operator’s loopback). Validate does **not** reject pairing `"*"` with `dev-loopback-unauth` in 1.0.

### D18k — Live-read snapshot; not racy `reloadOrigins`

Adapters **live-read** `OriginAllowlist` from the active snapshot on every request (same pattern as `UIEnabled` in `cmd/labmail/serve.go`). They do **not** copy the slice on `OnReset`/`OnApply`. `cfg.AllowedOrigins` is the test-only fallback when no getter is set.

`snapshot.Store` is `atomic.Pointer`. Canonical spec is immutable after swap. Unsynchronized `s.cfg.AllowedOrigins = …` from reset hooks races with `serveHTTP` (`make test-race`). `reloadAuth` is locked inside `Verifier.Replace`; there is no equivalent for a slice. A missed origin reload is an availability bug (LAN UI stays 403). Production wires one closure over `*app.App` passed to REST, MCP, and compat; tests keep mutating `AllowedOrigins` with the getter nil. Do not type-assert `s.svc.(*app.App)` inside adapters.

### D18l — MCP `DisableLocalhostProtection` stays true

MCP Streamable HTTP keeps `DisableLocalhostProtection: true` in `internal/control/mcp/server.go`. Origin policy is `auth.CheckOrigin` only. This work must not flip that flag. Re-enabling the SDK check would make `"*"` / `"private"` appear to work on REST SPA assets and still 403 MCP from non-loopback `RemoteAddr` (Codespaces).

`isLoopbackHost` stays `h == "localhost"` (case-sensitive) or `net.ParseIP(host) != nil && ip.IsLoopback()`. D18 does **not** case-fold `localhost`. `X-Forwarded-For` is not trusted. Cookie `Secure` / `SameSite=Lax` / CSRF rules are unchanged.

### Matching algorithm (`auth.CheckOrigin`)

Normative steps, in order (implementation follows this ADR; not claimed shipped here):

1. Trim space. Empty Origin → `nil` (allowed).
2. `url.Parse`. On error, empty host, or scheme not `http`/`https` (lowercase) → `forbidden` `"origin is not allowed"`. `file://` dies here even if host is `localhost`. Literal `Origin: null` dies here **even if** the allowlist contains `"*"`.
3. If `isLoopbackHost(u.Hostname())` → `nil`. Helper unchanged.
4. Walk `allowlist` (nil treated as empty). For each entry:
   - `raw := strings.TrimSpace(allowed)`.
   - If `raw == "*"` → `nil` (http(s) already enforced in step 2). **Do not** slash-strip before sentinel detection (`"*/"` is not the any-origin hatch at match time; validate already rejects it).
   - If `strings.EqualFold(raw, "private")` and the Origin host parses as an IP with `ip.IsPrivate()` → `nil`.
   - Else if existing `originMatches` (EqualFold of trimmed, slash-stripped **full Origin strings**) → `nil`.
5. Deny.

Do **not** rebuild allowlist entries with `url.URL.String()` (IPv6 brackets, encoding, empty-port forms can disagree with what the browser sends). Exact-match remains match-time EqualFold after trim + trailing-slash strip. No path, query, or userinfo is considered for sentinels.

Normalize does **not** rewrite `originAllowlist` entries (no trailing-`/` strip into the stored spec, no `PRIVATE` → `private`, no parse-and-rebuild). Rewriting would churn `runtimeRevision` on upgrade-without-YAML-edit. Empty strings stay in the list so Validate can emit `spec.management.originAllowlist[i]`. Duplicates stay.

### Validate procedure

Today any string loads. After D18, validate is fail-closed. This **is** a compatibility break for YAML that contained junk or previously harmless exact-miss strings (including a parked `https://*.github.dev`).

**Exception to the 04 additive-fields promise.** [docs/04-state-and-configuration.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/04-state-and-configuration.md) says: `labmail.dev/v1alpha1` is fail-closed; additive fields only after schema bump or explicit defaulting ADR. This ADR **is** that ADR for the **existing** field `spec.management.originAllowlist`. It blesses:

1. Additive **sentinel semantics** on the existing strings `"*"` and `"private"` (no schema bump, no new field).
2. **Fail-closed validate** of values that previously loaded (junk, globs, empty entries, non-http(s)). Same family as reserved-key reject. It is not “neither a new field nor a default change.”

No schema bump. Stored legal exact entries are not rewritten, so upgrade-without-YAML-edit of a legal exact list does not churn `sha256:`.

Normative procedure (do not invent a second table):

1. **Cap 64** — if `len(originAllowlist) > 64` → one violation `spec.management.originAllowlist` / `invalid_value` / `originAllowlist cap 64`. Then for each index `i`:
2. **Trim** — `raw := strings.TrimSpace(entry)`.
3. **Empty reject** — if `raw == ""` → `spec.management.originAllowlist[i]` / `invalid_value` / `empty originAllowlist entry`.
4. **Exact `"*"`** — if `raw == "*"` → OK (sentinel).
5. **EqualFold `private`** — if `strings.EqualFold(raw, "private")` → OK (sentinel).
6. **Became-sentinel reject** — `stripped := strings.TrimRight(raw, "/")`. If `stripped == "*"` **or** `strings.EqualFold(stripped, "private")` → `invalid_value` / `originAllowlist sentinel must be exactly * or private` (so `"*/"` and `"private/"` cannot fail-open into a hatch).
7. **Other wildcards** — if `strings.Contains(raw, "*")` → `invalid_value` / `wildcards other than the sentinel * are not supported`. ASCII `*` only; do not scan fullwidth `＊` (it fails the URL-shape check).
8. **`url.Parse`** — `u, err := url.Parse(raw)`. On error → `invalid_value`.
9. **http(s) shape** — scheme must be `http` or `https` (lowercase compare). Host (`u.Hostname()` or `u.Host`) non-empty. `u.User == nil`. `u.RawQuery == ""`. `u.Fragment == ""`. Path empty or `/` only.
10. **No rebuild** — otherwise OK (exact Origin). Do **not** rebuild with `u.String()`.

Do **not** require that `"*"` be the sole entry. Go Validate remains SoT. JSON Schema may describe `maxItems: 64` and the string items; it must not become an enum (that would reject exact Origins).

Migration (implementation CHANGELOG **Changed**, one line): replace non-http(s) / glob / empty `originAllowlist` entries with exact `http(s)://host[:port]`, `"*"`, or `"private"`. Bootstrap and reset **fail** on the old junk (`validation_failed`). YAML that already contains `"*"` expecting a no-op exact miss will **start allowing all http(s) Origins** once the matcher ships.

## Consequences

- Remote-dev / Codespaces can set `originAllowlist: ["*"]` (quoted) and keep `bearer_and_basic`. LAN DHCP can set `["private"]` or an exact Origin. Published LAN with a stable Origin should prefer exact.
- Default empty allowlist still 403s non-loopback SPA JS until the operator hatches. That residual belongs in `docs/known-limitations.md` **in the same change as the matcher**, not in this ADR-only change.
- `"*"` disables DNS-rebinding origin defense for all http(s) Origins. Do not present it as safe for a production published `:1080`. `"private"` is narrower (`https://evil.example` still denied) but allows any RFC1918 / ULA / CGNAT Origin host, including Tailscale `100.x`, not only the bind address.
- OPTIONS stays 403 with hatches on. Tests in the implementation PR must assert that so CORS is not “fixed” in to load the SPA.
- Adapters share one live-read closure; a REST-only getter would leave `GET /email` and `POST /mcp` on the bind-time list.
- Previously loaded junk YAML fails bootstrap after the validate change. That is accepted.
- Numbered pack, cookbook, examples, CHANGELOG, and code land with the matcher (follow-up PR). This ADR must not be treated as proof that `"*"` already matches.

## Alternatives considered

### A. Always-on RFC1918/ULA auto-allow (no YAML)

LAN UI would work with zero config. Weakens default-deny on every published LAN `:1080` without an audit trail; does not help Codespaces/public DNS. **Rejected.** Use opt-in `"private"`.

### B. New boolean `spec.management.allowAnyOrigin` / `devMode`

Obvious knob. Second surface besides the list; `devMode` invites disabling auth. **Rejected.**

### C. Host wildcards `https://*.github.dev`

Narrower than `"*"` for Codespaces. Suffix/dot attacks, IDNA, a new matcher; Codespaces URLs still vary. **Out of 1.0.** Validate-reject any entry containing `*` other than the sentinel so a later ADR can add globs without silent partial matches.

### D. CORS success path in 1.0 (`Access-Control-Allow-Origin` echo, OPTIONS 204)

Would unblock Vite `:5173` **without** proxy talking to `:1080`. New protocol surface; CSRF/credential interactions; does not fix same-origin hashed JS; frozen docs say no CORS. **Out of 1.0.** Revisit only with a dedicated ADR if a first-party cross-origin UI exists.

### E. Strip Vite `crossorigin` / use classic scripts

Might drop Origin on some browsers for CSS. Module scripts still CORS-mode; does not help `/v1` fetches from a non-loopback page Origin. **Rejected.**

### F. Allow when `Origin` host equals `Host` header

Same-origin UI would work on LAN and remote DNS without YAML. Classic DNS rebinding: attacker controls DNS for `Host` and `Origin` together. This is the attack the allowlist exists to stop. **Rejected.**

### G. `"*"` only; no `"private"`

Smaller matcher. LAN operators who will not accept `https://evil.example` have only exact DHCP IPs. **Rejected.** Include `"private"` in 1.0 as a same-field sentinel.

### H. Live apply op `replaceOriginAllowlist`

No inbox wipe to change origins. Expands the frozen apply table; origin is a security control that GitOps YAML + reset already models. **Out of 1.0.** Reset/restart only.

### I. Copy allowlist on `OnReset`/`OnApply` (`reloadOrigins`) vs live snapshot read

Copy-on-hook would mirror `reloadAuth`. Data race with `serveHTTP` unless a mutex/`atomic.Pointer[[]string]` is added; `app.Service` has no `Active()` so a type assert can no-op; three helpers can drift so REST has `"*"` and MCP/compat still 403. **Rejected.** Live-read (D18k). `cfg.AllowedOrigins` remains the nil-getter test knob.

## Review triggers

Review this decision when CORS or OPTIONS success is proposed, when host globs are proposed, when a live apply op for origin is proposed, when `DisableLocalhostProtection` would be flipped, when `isLoopbackHost` would case-fold `localhost`, or when always-on private-IP allow is proposed. A CORS request relative to D18f is **wontfix** unless a new ADR supersedes this one.
