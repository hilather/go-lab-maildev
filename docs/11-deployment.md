# Deployment

Status: Proposed
Last reviewed: 2026-08-17

## Binary

`labmaild serve` is the only long-running command. Distroless or Alpine runtime; no Node.

Multi-stage Dockerfile:

1. Node: build `web/dist`.
2. Go: `CGO_ENABLED=0` build with embed.
3. Runtime: copy binary, `USER 65532`, expose 1025 and 1080.

Pin base images by digest in release files.

## Reference Compose (this repo)

```yaml
services:
  labmail:
    image: ghcr.io/hilather/go-lab-maildev:TODO
    command: ["serve", "--config=/etc/labmail/config.yaml"]
    ports:
      - "1025:1025"
      - "1080:1080"
    volumes:
      - ./config/examples/lab.yaml:/etc/labmail/config.yaml:ro
      - ./secrets/web-password:/run/secrets/web-password:ro
      - ./secrets/token:/run/secrets/token:ro
    read_only: true
    tmpfs: ["/tmp"]
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    user: "65532:65532"
    healthcheck:
      test: ["CMD", "/labmaild", "healthcheck", "--smtp=127.0.0.1:1025", "--url=http://127.0.0.1:1080/v1/health/ready"]
```

MailDev-flag mode for lab cutover:

```text
command: ["serve", "--smtp", "1025", "--web", "1080"]
environment:
  MAILDEV_WEB_USER: admin
  MAILDEV_WEB_PASS: ${MAILDEV_WEB_PASS}
```

## Comparison lab (this repo)

Side-by-side MailDev oracles live in `deploy/parity-lab/` ([22-comparison-lab.md](22-comparison-lab.md)). That compose is **not** the mcp-integration-lab cutover file. Product examples keep outgoing off. Parity-lab `relay` / `autorelay` profiles may set `outgoing-host=relay-sink` only.

## Kubernetes

A later example: two Service ports, Secret for passwords, read-only root, drop caps. Not GA-blocking.

## Resource guidance

Small: 64–128 MiB RAM for empty inbox. Bound `maxMessages` and `maxMessageSize` so a flood cannot unbounded-grow RSS.
