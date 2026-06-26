# AtlasGraph - Economic Shock Propagation Engine
# Common developer tasks. Run `make help` for the list.

BINARY      := atlas
PKG         := github.com/atlasgraph/atlas
CMD         := ./cmd/atlas
BIN_DIR     := bin
GO          ?= go

# Embed version/build metadata into the binary at link time.
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)
LDFLAGS     := -X '$(PKG)/internal/config.Version=$(VERSION)' \
               -X '$(PKG)/internal/config.Commit=$(COMMIT)' \
               -X '$(PKG)/internal/config.BuildDate=$(DATE)'

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the atlas binary into ./bin
	@$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)

.PHONY: install
install: ## Install the atlas binary into $GOBIN
	@$(GO) install -ldflags "$(LDFLAGS)" $(CMD)

.PHONY: run
run: ## Run the sample shock scenario
	@$(GO) run $(CMD) shock --source Taiwan --commodity semiconductors --drop 30 --depth 3

.PHONY: scenarios
scenarios: ## List the bundled scenario presets
	@$(GO) run $(CMD) scenario list

.PHONY: summary
summary: ## Print graph summary statistics
	@$(GO) run $(CMD) graph summary

.PHONY: leaderboard
leaderboard: ## Print the baseline fragility leaderboard
	@$(GO) run $(CMD) risk leaderboard

.PHONY: test
test: ## Run all unit tests
	@$(GO) test ./...

.PHONY: cover
cover: ## Run tests with coverage report
	@$(GO) test -cover ./...

.PHONY: vet
vet: ## Run go vet static analysis
	@$(GO) vet ./...

.PHONY: fmt
fmt: ## Format the codebase
	@$(GO) fmt ./...

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum
	@$(GO) mod tidy

.PHONY: check
check: fmt vet test ## Format, vet and test

.PHONY: clean
clean: ## Remove build artifacts
	@rm -rf $(BIN_DIR) dist coverage.out
