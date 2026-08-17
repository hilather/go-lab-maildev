# Wave CMP — Side-by-side MailDev comparison lab

Status: not-started
Dependencies: none to start oracle+harness against MailDev; LabMail rows need W4-DEP (image) and W3-REST/W3-UI
Parallel: **can start the day wave 1 lands** (even alongside wave 2/3). Exclusive tree: `deploy/parity-lab/**`, `test/parity-lab/**`

Read: [22-comparison-lab.md](../22-comparison-lab.md), [23-behavior-parity-matrix.md](../23-behavior-parity-matrix.md), [ADR 0013](../adr/0013-full-maildev-parity-and-comparison-lab.md)

This wave is the **oracle**. Do not wait for LabMail to characterize MailDev.

## CMP-ORACLE — Compose oracles

Exclusive: `deploy/parity-lab/**` except the `labmail` service block (CMP-SUT owns that). Includes `compose.yaml`, overlays, `pins.env`, `lint-outgoing-host.sh`, `README.md`.

### Goal

`docker compose --profile oracle up` runs `maildev-v2` (2.2.1 digest-pinned), `maildev-v3` (git SHA build), `relay-sink` (2.2.1, no outgoing). Host ports 21025/21080, 31025/31080, 51025/51080. Health: HTTP `/healthz` or `/api/healthz`.

### Scope

- [ ] Pin file with image digest + git SHA
- [ ] Profiles `oracle`, `sut`, `harness` on `compose.yaml`
- [ ] Overlays: `compose.relay.yaml`, `compose.autorelay.yaml`, `compose.auth.yaml`, `compose.basepath.yaml`, `compose.directory.yaml`, `compose.https.yaml`, `compose.smtps.yaml`, `compose.disableweb.yaml`
- [ ] Relay/autorelay overlays set outgoing-host **only** `relay-sink`
- [ ] `lint-outgoing-host.sh` fails if outgoing-host is anything else
- [ ] README: how to open the three UIs; Make targets from docs/22

### Required tests

- [ ] `docker compose config` 
- [ ] Smoke: SMTP to v2 and v3, GET list returns the subject (oracle-only CI job)

---

## CMP-REST — REST characterization + dual compare

Exclusive: `test/parity-lab/rest/**`, `test/parity-lab/goldens/**`

### Goal

Go tests implement matrix S*, R*, Y* via SMTP + HTTP. Normalization in `normalize.go`. With only oracles up, v2/v3 rows run. With `sut` profile, LabMail rows are required.

### Required tests

- [ ] All REST rows in the matrix (skip lm until image exists; skip Y* unless relay overlay; skip Y5–Y7 unless autorelay overlay; skip P2–P6 unless matching overlay)
- [ ] Golden update is explicit (`-update-goldens`), never silent in CI
- [ ] Dual prefix assertion for LabMail when SUT present
- [ ] `lint-outgoing-host` in the same job

---

## CMP-UI — Playwright workflows

Exclusive: `test/parity-lab/ui/**`

### Goal

Shared specs + `oracles/v2.ts` + `oracles/v3.ts`. LabMail uses v3 adapters against port 41080. Matrix L*, U*, and UI columns of R*/Y*.

### Required tests

- [ ] Inbox list after SMTP (v2, v3; lm when up)
- [ ] Open HTML/text/headers/source
- [ ] Delete one / delete all
- [ ] Live new mail without reload
- [ ] Relay click with `relay` profile: message on relay-sink (REST check from the test after UI click)
- [ ] Auth profile: UI blocked without credentials

Screenshots on failure only.

---

## CMP-SUT — Wire LabMail into compose

Dependencies: W4-DEP image, W3-WIRE
Exclusive: `deploy/parity-lab/compose.yaml` `labmail` service only (coordinate with CMP-ORACLE)

### Goal

`--profile sut` builds/runs LabMail. `make test-parity-lab` fails if LabMail diverges after normalization.

---

## CMP-CI — Make + GitHub job

Exclusive: `Makefile` targets listed in docs/22, `.github/workflows` job `parity-lab`

### Goal

- `make test-parity-lab-oracle` on PRs always (MailDev only; catches harness rot).
- `make test-parity-lab` required once `labmail` image build is in CI.
- No public SMTP credentials in the workflow.

### Required tests

- [ ] Job uses compose files; relay and autorelay are separate matrix cells
- [ ] Overlay cells for basepath, directory, https, smtps, disableweb, auth
- [ ] Hardening: if flake, fix wait/poll in harness, do not skip rows

---

## Wave CMP definition of done

Matrix `req` cells pass for v2, v3, and LabMail. GA cannot claim MailDev parity without this wave green.

Handoff: update [23-behavior-parity-matrix.md](../23-behavior-parity-matrix.md) notes only if a MailDev quirk is documented; do not weaken asserts to match a LabMail bug.
