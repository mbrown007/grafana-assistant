.PHONY: help build run test clean docker-build docker-run install deps fmt lint

# Variables
BINARY_NAME=alertmanager-mcp-server
DOCKER_IMAGE=alertmanager-mcp-go
DOCKER_TAG=latest
MAIN_PATH=./cmd/server

help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

build: deps ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) $(MAIN_PATH)

run: ## Run the server (stdio mode)
	@echo "Running $(BINARY_NAME) with stdio transport..."
	go run $(MAIN_PATH)

run-sse: ## Run the server with SSE transport
	@echo "Running $(BINARY_NAME) with SSE transport..."
	go run $(MAIN_PATH) -transport sse -host 0.0.0.0 -port 8000

test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

coverage: test ## Show test coverage
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f coverage.out
	go clean

docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run: ## Run Docker container (stdio mode)
	@echo "Running Docker container..."
	docker run --rm -i \
		-e ALERTMANAGER_URL=${ALERTMANAGER_URL} \
		-e ALERTMANAGER_USERNAME=${ALERTMANAGER_USERNAME} \
		-e ALERTMANAGER_PASSWORD=${ALERTMANAGER_PASSWORD} \
		-e ALERTMANAGER_TENANT=${ALERTMANAGER_TENANT} \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

docker-run-sse: docker-build ## Run Docker container with SSE transport
	@echo "Running Docker container with SSE transport..."
	docker run --rm \
		-e ALERTMANAGER_URL=${ALERTMANAGER_URL} \
		-e ALERTMANAGER_USERNAME=${ALERTMANAGER_USERNAME} \
		-e ALERTMANAGER_PASSWORD=${ALERTMANAGER_PASSWORD} \
		-e ALERTMANAGER_TENANT=${ALERTMANAGER_TENANT} \
		-e MCP_TRANSPORT=sse \
		-p 8000:8000 \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

install: build ## Install binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	go install $(MAIN_PATH)

.DEFAULT_GOAL := help
