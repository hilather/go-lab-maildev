# Wave 5 — GA hardening

Status: not-started
Dependencies: wave 4 **and** wave CMP (`make test-parity-lab` green)
Parallel: W5-INTEROP-* dirs; serial W5-REL then W5-GA

Read: [19-acceptance-criteria.md](../19-acceptance-criteria.md), [14-release-engineering.md](../14-release-engineering.md), [AGENTS.md](../../AGENTS.md) §2.5–2.6

## W5-INTEROP-GO / PY / NODE / JAVA

Exclusive: `test/interop/<lang>/**` each

### Goal

Submit mail with that ecosystem’s common library; assert REST get. Document client settings (no STARTTLS required).

### Required tests

- [ ] One successful submit per language in CI (Java optional if image-heavy; if skipped, known-limitations + scheduled job)

Suggested: Go `net/smtp` (already), Python `smtplib`, Node `nodemailer`, swaks shell.

---

## W5-REL — Release automation

Exclusive: `scripts/release-*`, `Makefile` release targets, `.github/workflows/release.yml`, changelog linter

### Goal

`make test-changelog`, release notes template enforcement, tag-gate workflow. Compare OpenAPI/MCP/schema between refs when those files exist.

### Required tests

- [ ] Missing CHANGELOG section fails
- [ ] Template TODO in notes file fails

---

## W5-SECSCAN

Exclusive: workflow + `Makefile` `security-scan`

### Goal

`govulncheck` and secret scan on CI. No high/critical without waiver doc.

---

## W5-GA — Acceptance integrator

Dependencies: all W5 tasks that are in-scope
Exclusive: `docs/releases/v1.0.0-rc.1.md`, `docs/releases/acceptance-evidence.md`, program board status, CHANGELOG version section

### Goal

Tick [19-acceptance-criteria.md](../19-acceptance-criteria.md) with links to tests. Do not tag unless instructed; prepare the SHA.

### Required process

- [ ] All required CI green on the candidate SHA
- [ ] Watch the PR chain’s last PR and `main` ([AGENTS.md](../../AGENTS.md) §2.6)
- [ ] Hardening records for any CI failures during the train
- [ ] Complete between-version notes vs repository start
- [ ] `make test-parity-lab` evidence (REST + UI vs MailDev v2 and v3)

## Wave 5 definition of done

M5. Product is a GA candidate. mcp-integration-lab cutover PR can open against a tagged image.
