# LabMail task runner. Tool versions are pinned; do not use @latest.

GO ?= go
export GOTOOLCHAIN ?= local
export GOPROXY ?= https://proxy.golang.org,direct

GOLANGCI_LINT_VERSION ?= v2.12.2
GOVULNCHECK_MOD ?= golang.org/x/vuln/cmd/govulncheck@v1.1.4
GOLANGCI_LINT_MOD ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: help fmt format lint vet build generate verify-generated test test-race \
	test-fuzz-smoke test-parity test-config-compat test-docs test-container \
	security-scan test-changelog

help:
	@printf '%s\n' \
		'LabMail Make targets (Go 1.26; module github.com/hilather/go-lab-maildev)' \
		'  format              go fmt ./...' \
		'  fmt                 alias for format' \
		'  vet                 go vet ./...' \
		'  lint                go vet + golangci-lint $(GOLANGCI_LINT_VERSION)' \
		'  build               go build -o bin/labmail ./cmd/labmail' \
		'  generate            unimplemented until API-001 (PR 6); fail-closed' \
		'  verify-generated    unimplemented until API-001 (PR 6); fail-closed' \
		'  test                go test ./...' \
		'  test-race           go test -race ./...' \
		'  test-fuzz-smoke     execute the buildinfo seed corpus' \
		'  test-docs           required documents, metadata, and links' \
		'  security-scan       govulncheck' \
		'  test-parity         unimplemented until MCP-001 (PR 8); fail-closed' \
		'  test-config-compat  unimplemented until CFG-001 (PR 2); fail-closed' \
		'  test-container      unimplemented until DEP-001 (PR 11); fail-closed' \
		'  test-changelog      unimplemented until REL/GA; fail-closed'

fmt: format

format:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint: vet
	$(GO) run $(GOLANGCI_LINT_MOD) run ./...

build:
	$(GO) build -o bin/labmail ./cmd/labmail

generate:
	@echo 'generate: unimplemented until API-001 (PR 6)' >&2; exit 1

verify-generated:
	@echo 'verify-generated: unimplemented until API-001 (PR 6)' >&2; exit 1

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-fuzz-smoke:
	$(GO) test ./internal/buildinfo -fuzz=FuzzInfoString -fuzztime=5s -count=1

test-docs:
	$(GO) run ./scripts/checkdocs

security-scan:
	$(GO) run $(GOVULNCHECK_MOD) ./...

test-parity:
	@echo 'test-parity: unimplemented until MCP-001 (PR 8)' >&2; exit 1

test-config-compat:
	@echo 'test-config-compat: unimplemented until CFG-001 (PR 2)' >&2; exit 1

test-container:
	@echo 'test-container: unimplemented until DEP-001 (PR 11)' >&2; exit 1

test-changelog:
	@echo 'test-changelog: unimplemented until a checkchangelog script lands' >&2; exit 1
