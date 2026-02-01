.PHONY: build run

BINARY_NAME := kb-mcp-server

build:
	go build -o $(BINARY_NAME) ./cmd/server

run: build
	./$(BINARY_NAME)
