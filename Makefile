BINARY      := modjo
PKG         := github.com/tdeschamps/modjo-cli
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG := $(PKG)/internal/cmd/version
LDFLAGS     := -s -w \
	-X $(VERSION_PKG).Version=$(VERSION) \
	-X $(VERSION_PKG).Commit=$(COMMIT) \
	-X $(VERSION_PKG).Date=$(DATE)

# Coverage gate: total statement coverage must stay at or above this.
COVER_MIN   ?= 95.0
COVERPKG    := ./internal/...,./cmd/...

# Use the locally installed toolchain (avoids partial auto-downloaded toolchains).
export GOTOOLCHAIN ?= local

.PHONY: all build test test-e2e cover cover-html lint fmt fmt-check vet vuln tools snapshot docs clean tidy

all: fmt-check lint test

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/modjo

test:
	go test -race ./...

test-e2e:
	go test -race ./cmd/modjo/ -run TestScripts -v

cover:
	go test -coverprofile=cover.out -covermode=atomic -coverpkg=$(COVERPKG) ./...
	@go tool cover -func=cover.out | tail -1
	@total=$$(go tool cover -func=cover.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	awk -v t=$$total -v m=$(COVER_MIN) 'BEGIN { if (t+0 < m+0) { printf "FAIL: coverage %.1f%% < %.1f%%\n", t, m; exit 1 } else { printf "OK: coverage %.1f%% >= %.1f%%\n", t, m } }'

cover-html: cover
	go tool cover -html=cover.out -o cover.html
	@echo "wrote cover.html"

lint:
	golangci-lint run ./...

fmt: tools
	gofumpt -w ./internal ./cmd
	goimports -w ./internal ./cmd

fmt-check: tools
	@out=$$(gofumpt -l ./internal ./cmd); \
	if [ -n "$$out" ]; then echo "gofumpt needed on:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

vuln: tools
	govulncheck ./...

# Install the developer tooling this project standardizes on. gofumpt is pinned
# to match CI exactly (and to stay buildable on the go 1.24 baseline).
tools:
	@command -v gofumpt >/dev/null      || go install mvdan.cc/gofumpt@v0.7.0
	@command -v goimports >/dev/null    || go install golang.org/x/tools/cmd/goimports@latest
	@command -v govulncheck >/dev/null  || go install golang.org/x/vuln/cmd/govulncheck@latest

snapshot:
	goreleaser release --snapshot --clean

docs:
	go run ./cmd/modjo docs

tidy:
	go mod tidy

clean:
	rm -rf bin dist cover.out cover.html
