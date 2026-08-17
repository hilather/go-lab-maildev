# ADR 0010: YAML primary, MailDev flags overlay

Status: Accepted
Date: 2026-08-17

## Context

Sibling appliances are YAML-configured. The lab currently renders MailDev CLI flags from `maildev.yaml`. A hard YAML-only cutover would block drop-in.

## Decision

Canonical config is `labmail.dev/v1alpha1` YAML. CLI flags and `MAILDEV_*` env overlay with MailDev names. Relay flags/keys fail closed. Merge order: flags > env > file > defaults.

## Alternatives considered

- Flags only — poor GitOps.
- YAML only — extra lab PR before the image can land.

## Consequences

Two config languages to test; one internal `model.Config`.

## Compatibility impact

`MAILDEV_ARGS=--smtp 1025 --web 1080` works.

## Migration

Lab can later mount YAML like LabDNS.

## Test impact

Table tests for every accepted and rejected flag.

## Documentation impact

docs/04, 12.

## Review triggers

Lab drops flag mode.
