.PHONY: all build run run-sse clean test docker-build docker-run

# Variables
BINARY_NAME=genesys-mcp-server
DOCKER_IMAGE=genesys-cloud-mcp-go
VERSION?=1.0.0

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o bin/$(BINARY_NAME) ./cmd/server

run: build
	@echo "Running MCP server (stdio mode)..."
	@./bin/$(BINARY_NAME)

run-sse: build
	@echo "Running MCP server (SSE mode)..."
	@MCP_TRANSPORT=sse ./bin/$(BINARY_NAME)

clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@go clean

test:
	@echo "Running tests..."
	@go test -v ./...

deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

docker-run:
	@echo "Running Docker container (stdio mode)..."
	@docker run --rm -it --env-file .env $(DOCKER_IMAGE):latest

docker-run-sse:
	@echo "Running Docker container (SSE mode)..."
	@docker run --rm -it --env-file .env -p 8080:8080 \
		-e MCP_TRANSPORT=sse \
		$(DOCKER_IMAGE):latest

lint:
	@echo "Running linter..."
	@golangci-lint run

fmt:
	@echo "Formatting code..."
	@go fmt ./...

help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  run         - Run in stdio mode"
	@echo "  run-sse     - Run in SSE mode"
	@echo "  clean       - Remove build artifacts"
	@echo "  test        - Run tests"
	@echo "  deps        - Install dependencies"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run  - Run Docker container (stdio)"
	@echo "  docker-run-sse - Run Docker container (SSE)"
	@echo "  lint        - Run linter"
	@echo "  fmt         - Format code"
