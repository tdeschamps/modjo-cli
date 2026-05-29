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

.PHONY: all build test test-e2e lint fmt vet vuln snapshot docs clean tidy

all: lint test build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/modjo

test:
	go test -race ./...

test-e2e:
	go test -race ./cmd/modjo/ -run TestScripts -v

lint:
	golangci-lint run ./...

fmt:
	gofmt -w ./internal ./cmd

vet:
	go vet ./...

vuln:
	govulncheck ./...

snapshot:
	goreleaser release --snapshot --clean

docs:
	go run ./cmd/modjo docs

tidy:
	go mod tidy

clean:
	rm -rf bin dist
