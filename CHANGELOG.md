# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

Land changes with an entry under `[Unreleased]`. Cutting a release promotes that section to a dated version heading so every release summarizes **all** high-level changes since the previous tag ([AGENTS.md](AGENTS.md) §2.5).

## [Unreleased]

### Added

- Architecture, evaluation, parity plan, ADRs, and wave task lists for the LabMail Go rewrite (`docs/`).
- Side-by-side Docker comparison lab design: original MailDev 2.2.1 + 3.0 vs LabMail, REST and UI behavior matrix, compose overlays for relay/auth/TLS/base-path (`docs/22-comparison-lab.md`, `docs/23-behavior-parity-matrix.md`, wave CMP).
- Mandatory agent instructions: regression tests, documentation-in-the-same-change, complete between-version release notes, and CI watch/hardening after PRs and PR chains (`AGENTS.md`, `.cursor/rules/`).

### Changed

- Full MailDev process parity (including relay/auto-relay) is in-scope; mcp-integration-lab deploy remains outgoing-off. ADR 0002/0011 superseded by ADR 0013.

### Fixed

- None.

### Removed or deprecated

- None.
