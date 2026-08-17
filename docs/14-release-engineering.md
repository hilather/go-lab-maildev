# Release engineering

Status: Proposed normative
Last reviewed: 2026-08-17
Related: [AGENTS.md](../AGENTS.md) §2.5–2.6, [RELEASE-NOTES-TEMPLATE.md](../RELEASE-NOTES-TEMPLATE.md)

## Versioning

SemVer for `labmaild`. Separately visible in `/v1/version`:

- Config `apiVersion`
- REST dual-prefix compatibility (MailDev 2/3)
- MCP protocol versions
- UI upstream commit

## Required CI (when wave 1 lands)

Every job required. No `continue-on-error` on merge gates. Actions pinned by commit SHA.

Suggested jobs: `format`, `lint`, `unit`, `race`, `fuzz-smoke`, `generated`, `docs`, `parity`, `parity-lab-oracle`, `config-compat`, `govulncheck`, `gitleaks`, `container`, `changelog`, `ci-gate`.

Once the LabMail image builds in CI, `parity-lab` (`make test-parity-lab`: REST + UI vs MailDev v2/v3) is required on PRs that touch SMTP, store, REST, UI, relay, or sanitizer, and on `main`. A separate matrix cell runs the `relay` / `autorelay` profiles. Oracle-only (`make test-parity-lab-oracle`) runs on every PR after wave CMP compose exists, even before LabMail.

UI jobs: `web-typecheck`, `web-lint`, `web-test`, `web-build`. Product Playwright on PRs if not too slow; comparison-lab Playwright is the `parity-lab` job above.

## Changelog and notes

1. Land user-visible work under `CHANGELOG.md` `[Unreleased]`.
2. Before tag: move that section to `## [vX.Y.Z] — YYYY-MM-DD`.
3. Write `docs/releases/vX.Y.Z.md` from the template. **All** high-level differences from the previous tag. `git log` may be appended as a supplement only.
4. `make test-changelog` fails if the tag notes file is missing headings or still contains `TODO`.

## Tag gate

Tag only the SHA whose required checks are green. After `git push origin vX.Y.Z`, wait for tag workflows. Red tag = blocker (move tag or patch tag). Record CI failures in `docs/ci-failure-hardening/`.

## PR and PR-chain watch

Agents must wait for GitHub Actions after every push. For a chain of PRs, watch the last PR’s `ci-gate`, then `main` after that merge. Fix and harden; do not leave `main` red. Details: [AGENTS.md](../AGENTS.md) §2.6.

## Artifacts

GitHub Release body = curated notes. Attach OpenAPI, MCP manifest, config schema, SBOM (`go list -m -json all`). Images: distroless default; optional distro variants later (not GA-blocking unless we claim them).

## Hardening after failures

Root-cause, fix, add a test or pin or timeout, write a hardening record. No unbounded retries. No skipped required jobs.
