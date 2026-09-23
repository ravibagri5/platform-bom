VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/ravibagri5/platform-bom/internal/cli.Version=$(VERSION)
EXAMPLE ?= examples/acme/pbom.yaml
GOLANGCI_LINT_VERSION ?= v2.13.2

.PHONY: all help ui build test test-coverage vet fmt tidy lint verify check run dev-ui kind-demo docker clean

all: ui build

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

ui: ## Build the web UI into internal/ui/dist
	cd web && npm ci && npm run build
	touch internal/ui/dist/.gitkeep

build: ## Build the pbom binary with the embedded UI
	go build -ldflags "$(LDFLAGS)" -o bin/pbom ./cmd/pbom

test: ## Run unit tests
	go test ./...

test-coverage: ## Run tests with coverage and print a summary
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

vet: ## Run go vet
	go vet ./...

fmt: ## Format Go code
	gofmt -w cmd internal

tidy: ## Tidy go.mod and go.sum
	go mod tidy

lint: ## Run golangci-lint
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

verify: ## Fail if go.mod is untidy or code is unformatted
	go mod tidy && git diff --exit-code -- go.mod go.sum
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)

check: verify vet lint test ## Everything CI runs

run: build ## Serve the example platform
	./bin/pbom serve -c $(EXAMPLE)

dev-ui: ## Run the Vite dev server (proxies /api to :8080)
	cd web && npm run dev

kind-demo: ## Create a kind cluster with Argo CD, cert-manager and Crossplane
	sh hack/kind-demo.sh

docker: ## Build the container image
	docker build -t platform-bom:$(VERSION) --build-arg VERSION=$(VERSION) .

clean: ## Remove build output
	rm -rf bin dist coverage.out web/node_modules
	find internal/ui/dist -mindepth 1 ! -name .gitkeep -delete
