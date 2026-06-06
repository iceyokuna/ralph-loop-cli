BINARY      := ralph
PKG         := ./cmd/ralph
MODULE      := github.com/iceyokuna/ralph-loop-cli

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(MODULE)/internal/cli.version=$(VERSION) \
           -X $(MODULE)/internal/cli.commit=$(COMMIT) \
           -X $(MODULE)/internal/cli.date=$(DATE)

.PHONY: build install test vet clean

build: ## Build a static binary into ./$(BINARY)
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

install: ## go install ralph onto $GOPATH/bin (or $GOBIN)
	CGO_ENABLED=0 go install -ldflags "$(LDFLAGS)" $(PKG)

test: ## Run the offline unit test suite
	go test ./...

vet: ## Run go vet
	go vet ./...

clean: ## Remove the built binary
	rm -f $(BINARY)
