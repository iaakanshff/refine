# refine — fast line dedup & sort.
# Common developer tasks. Run `make` or `make help` to see the targets.

BINARY  := refine
PKG     := ./...
GO      ?= go
GOFLAGS :=

.PHONY: all build install test test-race cover lint fmt vet fuzz bench clean help

all: build

## build: compile the refine binary into ./refine
build:
	$(GO) build $(GOFLAGS) -o $(BINARY) ./cmd/refine

## install: install refine into GOPATH/bin
install:
	$(GO) install $(GOFLAGS) ./cmd/refine

## test: run the unit tests
test:
	$(GO) test $(PKG)

## test-race: run the unit tests with the race detector
test-race:
	$(GO) test -race $(PKG)

## cover: produce a coverage report and print per-function totals
cover:
	$(GO) test -coverprofile=coverage.txt $(PKG)
	$(GO) tool cover -func=coverage.txt

## lint: run golangci-lint (install via `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
lint:
	golangci-lint run ./...

## fmt: gofmt + goimports across the tree
fmt:
	$(GO) fmt $(PKG)

## vet: go vet across the tree
vet:
	$(GO) vet $(PKG)

## fuzz: fuzz the deduplication core (override FUZZTIME, e.g. make fuzz FUZZTIME=2m)
FUZZTIME ?= 30s
fuzz:
	$(GO) test -fuzz=FuzzDeduplicate -fuzztime=$(FUZZTIME) ./internal/core

## bench: run all benchmarks with memory stats
bench:
	$(GO) test -run='^$$' -bench=. -benchmem ./...

## clean: remove build / coverage artifacts
clean:
	rm -f $(BINARY) coverage.txt

## help: show this help
help:
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
