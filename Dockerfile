# LabMail production image: ghcr.io/hilather/labmail
#
# Multi-stage, static binary, numeric non-root UID, no shell.
# Run with a read-only root filesystem, cap_drop ALL, and no-new-privileges.
# Container ports stay 1025 (SMTP) and 1080 (management). Healthcheck is HTTP
# ready via the copied binary. Ready is not an SMTP TCP connect.

FROM golang:1.26.6-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -trimpath \
	-ldflags="-s -w \
	-X github.com/hilather/go-lab-maildev/internal/buildinfo.version=${VERSION} \
	-X github.com/hilather/go-lab-maildev/internal/buildinfo.commit=${COMMIT} \
	-X github.com/hilather/go-lab-maildev/internal/buildinfo.buildTime=${BUILD_TIME}" \
	-o /out/labmail ./cmd/labmail \
	&& printf 'labmail:x:65532:65532:labmail:/:/sbin/nologin\n' > /out/passwd \
	&& printf 'labmail:x:65532:\n' > /out/group \
	&& cp /etc/ssl/certs/ca-certificates.crt /out/ca-certificates.crt \
	&& cp LICENSE /out/LICENSE

FROM scratch

LABEL org.opencontainers.image.title="labmail" \
	org.opencontainers.image.description="Receive-only SMTP lab appliance" \
	org.opencontainers.image.source="https://github.com/hilather/go-lab-maildev" \
	org.opencontainers.image.url="https://github.com/hilather/go-lab-maildev" \
	org.opencontainers.image.licenses="Apache-2.0" \
	org.opencontainers.image.vendor="hilather" \
	org.opencontainers.image.documentation="https://github.com/hilather/go-lab-maildev/blob/main/docs/11-deployment.md"

COPY --from=build /out/passwd /etc/passwd
COPY --from=build /out/group /etc/group
COPY --from=build /out/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/labmail /labmail
COPY --from=build /out/LICENSE /LICENSE

USER 65532:65532
EXPOSE 1025/tcp 1080/tcp
WORKDIR /

HEALTHCHECK --interval=10s --timeout=3s --start-period=3s --retries=3 \
	CMD ["/labmail", "healthcheck", "--url=http://127.0.0.1:1080/v1/health/ready"]

ENTRYPOINT ["/labmail"]
CMD ["serve", "--config=/etc/labmail/config.yaml"]
