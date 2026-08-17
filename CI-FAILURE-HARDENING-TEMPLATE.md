# CI failure hardening: YYYY-MM-DD short-slug

Date: TODO
Failed ref: TODO branch / tag / SHA
Failed run: TODO URL
Jobs: TODO names

## What failed

TODO: the user-visible symptom (timeout, flake, wrong assertion, missing tool, auth, etc.).

## Root cause

TODO: the actual defect in product, test, fixture, or pipeline. Not "CI was red".

## Immediate fix

TODO: the change that makes this SHA green.

## Hardening (required)

What now prevents the same class of failure from recurring silently? Pick at least one:

- [ ] Regression test that fails on the old behavior
- [ ] Fail-closed guard or schema check
- [ ] Pin (action SHA, module, image digest)
- [ ] Explicit timeout / wait / readiness
- [ ] Better assertion or diagnostic
- [ ] Hermetic fixture (no network, no wall-clock flake)
- [ ] Workflow change (required check, not a skip)

Describe the hardening:

TODO

## Follow-up

TODO: none, or a wave/task ID.
