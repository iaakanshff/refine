# =============================================================================
# refine — developer Makefile
#
#   make            → list targets
#   make check      → everything CI runs (fmt + vet + lint + test + build)
#   make release    → local GoReleaser snapshot (dist/, no upload)
#
# Overrideable:
#   make build VERSION=9.9.9-custom
#   make lint GOLANGCI_LINT_VERSION=v2.6.2
# =============================================================================

BINARY      := refine
PKG         := ./...
MAIN        := ./cmd/refine

MODULE      := $(shell go list -m)
CLI_PKG     := $(MODULE)/internal/cli

# Version from git describe (tag / commit / dirty) so dev builds are traceable.
# Release pipelines inject their own via ldflags. Leading 'v' is stripped
# because the CLI template renders its own.
VERSION ?= $(patsubst v%,%,$(shell git describe --tags --always --dirty 2>/dev/null || echo dev))

LDFLAGS     := -s -w -X $(CLI_PKG).version=$(VERSION)

GOLANGCI_LINT_VERSION ?= v2.6.2

.DEFAULT_GOAL := help
.PHONY: help all build test test-race cover fmt vet lint check tidy clean install uninstall release-snapshot

## help: show this help
help:
	@echo "refine — make targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

## all: fmt + vet + test + build (quick local loop)
all: fmt vet test build

## build: compile ./refine with version stamped from git
build:
	@echo "▶ building $(BINARY) $(VERSION)"
	@CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINARY) $(MAIN)

## install: build + place into $(GOPATH)/bin
install:
	@echo "▶ installing $(BINARY) $(VERSION)"
	@CGO_ENABLED=0 go install -trimpath -ldflags '$(LDFLAGS)' $(MAIN)

## test: run unit tests
test:
	@echo "▶ testing"
	@go test $(PKG) -count=1

## test-race: unit tests under the race detector
test-race:
	@echo "▶ race testing"
	@go test -race -count=1 $(PKG)

## cover: open an HTML coverage report in the browser
cover:
	@go test -coverprofile=coverage.out $(PKG) >/dev/null
	@go tool cover -html=coverage.out
	@rm -f coverage.out

## fmt: format every file in place
fmt:
	@echo "▶ gofmt"
	@go fmt ./...

## vet: static analysis
vet:
	@echo "▶ go vet"
	@go vet $(PKG)

## lint: golangci-lint via pinned version (system binaries drift)
lint:
	@echo "▶ golangci-lint ($(GOLANGCI_LINT_VERSION))"
	@go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

## check: the exact gate CI enforces — run before pushing
check: fmt vet lint test-race build
	@echo "✓ all checks passed"

## fuzz: fuzz the dedupe core (override FUZZTIME, e.g. make fuzz FUZZTIME=2m)
FUZZTIME ?= 30s
fuzz:
	@go test -fuzz=FuzzDeduplicate -fuzztime=$(FUZZTIME) ./internal/core

## bench: run all benchmarks with memory stats
bench:
	@go test -run='^$$' -bench=. -benchmem ./...

## clean: remove build artifacts
clean:
	@rm -rf dist coverage.out $(BINARY)
	@echo "✓ clean"
