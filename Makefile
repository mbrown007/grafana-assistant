.PHONY: build run test lint clean help

BIN := bin/assistant

build: ## Build the binary
	go build -o $(BIN) ./cmd/assistant

run: build ## Build and run
	./$(BIN)

test: ## Run tests
	go test ./...

lint: ## Run linter
	golangci-lint run

clean: ## Remove build artifacts
	rm -rf bin/

help: ## Show this help
	@grep -E '^[a-z][a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
