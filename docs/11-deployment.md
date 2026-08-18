# Deployment

Status: Proposed normative behavior
Owners: Platform, Operations
Last reviewed: 2026-08-17 (SMTP-001a)
Related ADRs: 0001, 0003

Dockerfile, compose, and `scripts/test-container.sh` land in DEP-001 (PR 11). This document freezes the contract so later PRs do not invent ports or image posture.

## CLI

```text
labmail serve --config=/etc/labmail/config.yaml
              [--smtp-listen ADDR] [--management-listen ADDR|off]
              [--shutdown-timeout 5s] [--pid-file /tmp/labmail.pid]
labmail validate --config=...
labmail canonicalize --config=... [--format yaml|json]
labmail healthcheck --url=http://127.0.0.1:1080/v1/health/ready
labmail version
```

`serve` loads → compile → bind SMTP → bind management → write pid file. Invalid bootstrap does **not** bind SMTP or management.

`SIGTERM`/`SIGINT`: stop SMTP accept, drain sessions (deadline), then HTTP, then `store.Wipe` spill files. `SIGUSR1` unused (no chaos).

`labmail send` is **not** shipped.

CFG-001 implements `version`, `help`, `validate`, and `canonicalize`. SMTP-001a implements `serve` for the SMTP listener only (Null sink; management HTTP and `healthcheck` remain unimplemented).

## Hardened container

Dockerfile (LabDNS shape, Go 1.26.6-alpine → scratch):

```
USER 65532:65532
EXPOSE 1025/tcp 1080/tcp
ENTRYPOINT ["/labmail"]
CMD ["serve", "--config=/etc/labmail/config.yaml"]
HEALTHCHECK CMD ["/labmail", "healthcheck", "--url=http://127.0.0.1:1080/v1/health/ready"]
```

If the UI stage is present, add a Node **22.14.0** build stage (TacLab) that copies `web/dist` into `internal/web` before `go build`. UI contract (pages, EventSource + 3s poll, no Relay/send/compose): [docs/01-architecture.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/01-architecture.md#embedded-operator-ui).

Posture:

- Image `ghcr.io/hilather/labmail` (`:local` for compose builds, digest-pin in GitOps)
- Non-root UID `65532:65532`
- scratch/static, read-only root
- `cap_drop: ALL`, `no-new-privileges:true`
- tmpfs `/tmp` (optional spill under `/tmp/labmail-spill`)
- no shell, no Docker socket
- Container ports stay **1025 / 1080** (not LabDNS `:8080`)

## Reference compose

Also `examples/compose.smoke.yaml` (DEP-001):

```yaml
services:
  labmail:
    image: ghcr.io/hilather/labmail:local
    command: ["serve", "--config=/etc/labmail/config.yaml"]
    ports:
      - "1025:1025/tcp"
      - "1080:1080/tcp"
    volumes:
      - ./labmail.yaml:/etc/labmail/config.yaml:ro
      - ./secrets/labmail-token:/run/secrets/labmail-token:ro
      - ./secrets/maildev-web-password:/run/secrets/maildev-web-password:ro
    read_only: true
    tmpfs: ["/tmp"]
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    user: "65532:65532"
    restart: unless-stopped
    networks: [default]
```

## Integration-lab compose fragment

Service name stays `maildev` (D15). Healthcheck plane change: today’s maildev probe is SMTP TCP via `node -e connect(1025)`. Scratch has no `node`; ready becomes HTTP `/v1/health/ready` (which still requires the SMTP listener bound). `depends_on` / start_period stay 3s / 12 retries.

```yaml
  maildev:   # service name kept for one release so depends_on / docs survive
    image: ghcr.io/hilather/labmail:<pin>
    command: ["serve", "--config=/etc/labmail/config.yaml"]
    networks: [default]
    ports:
      - "${MAILDEV_SMTP_PORT:-1025}:1025/tcp"
      - "${MAILDEV_WEB_PORT:-1080}:1080/tcp"
    volumes:
      - ${MCPLAB_PROFILE_DIR:-./profiles/default}/labmail/bootstrap.yaml:/etc/labmail/config.yaml:ro
      - ./secrets/labmail-token:/run/secrets/labmail-token:ro
      - ./secrets/maildev-web-password:/run/secrets/maildev-web-password:ro
    # Do not leave MAILDEV_ARGS / MAILDEV_WEB_USER / MAILDEV_WEB_PASS
    # here. `environment: {}` does not clear operator env — delete the
    # keys from the service definition entirely.
    read_only: true
    tmpfs: ["/tmp"]
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    user: "65532:65532"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "/labmail", "healthcheck", "--url=http://127.0.0.1:1080/v1/health/ready"]
      interval: 5s
      timeout: 3s
      retries: 12
      start_period: 3s
```

Full swap bill of materials: [docs/13-integration-lab-swap.md](https://github.com/hilather/go-lab-maildev/blob/main/docs/13-integration-lab-swap.md).

## Compatibility promise

Compose ports 1025/1080 are the lab contract.
