.PHONY: build run test test-integration frontend-test test-all lint clean help dev dev-stop dev-restart dev-logs frontend-build mcp-build mcp-start mcp-stop package install-systemd

BIN := bin/assistant

build: ## Build the Go binary
	go build -o $(BIN) ./cmd/assistant

run: build ## Build and run (foreground)
	./$(BIN) -config config.yaml

test: ## Run all Go unit tests
	go test ./...

test-integration: ## Run integration tests (needs TEST_GRAFANA_URL)
	go test -tags=integration -count=1 -timeout=120s ./tests/integration/...

frontend-test: ## Run frontend Vitest tests
	npm --prefix frontend run test

test-all: test frontend-test ## Run Go unit + frontend tests

lint: ## Run linter
	golangci-lint run

clean: ## Remove build artifacts
	rm -rf bin/

# ---------------------------------------------------------------------------
# MCP servers
# ---------------------------------------------------------------------------

mcp-build: ## Build all MCP servers
	@echo "Building AlertManager MCP..."
	cd mcp_servers/alertmanager-mcp-go && go build -o ../../bin/mcp-alertmanager ./cmd/server
	@echo "Building Grafana MCP..."
	cd mcp_servers/mcp-grafana && go build -o ../../bin/mcp-grafana ./cmd/mcp-grafana
	@echo "Building Genesys Cloud MCP..."
	cd mcp_servers/genesys-cloud-mcp-go && go build -o ../../bin/mcp-genesyscloud ./cmd/server
	@echo "All MCP servers built."

mcp-start: mcp-stop mcp-build ## Start all MCP servers in SSE mode
	@echo "Starting AlertManager MCP on :8000..."
	MCP_TRANSPORT=sse MCP_PORT=8000 ALERTMANAGER_URL=$${ALERTMANAGER_URL:-http://localhost:9093} \
		./bin/mcp-alertmanager > .mcp-alertmanager.log 2>&1 & echo $$! > .mcp-alertmanager.pid
	@echo "Starting Grafana MCP on :8001..."
	./bin/mcp-grafana -transport sse -address localhost:8001 \
		> .mcp-grafana.log 2>&1 & echo $$! > .mcp-grafana.pid
	@echo "Starting Genesys Cloud MCP on :8002..."
	MCP_TRANSPORT=sse MCP_PORT=8002 \
		./bin/mcp-genesyscloud > .mcp-genesyscloud.log 2>&1 & echo $$! > .mcp-genesyscloud.pid
	@sleep 1
	@echo "MCP servers started."

mcp-stop: ## Stop all MCP servers
	@for name in alertmanager grafana genesyscloud; do \
		if [ -f .mcp-$$name.pid ]; then \
			kill $$(cat .mcp-$$name.pid) 2>/dev/null; \
			rm -f .mcp-$$name.pid; \
			echo "Stopped $$name MCP"; \
		fi; \
	done

# ---------------------------------------------------------------------------
# Dev workflow — runs MCP servers + Go backend + Vite frontend
# ---------------------------------------------------------------------------

dev: dev-stop build ## Start everything for development
	@echo "=== Starting MCP servers ==="
	@$(MAKE) mcp-start --no-print-directory 2>/dev/null || echo "  (some MCP servers may have failed — check logs)"
	@sleep 1
	@echo ""
	@echo "=== Starting backend ==="
	./$(BIN) -config config.yaml > .dev-backend.log 2>&1 & echo $$! > .dev-backend.pid
	@echo "=== Starting frontend ==="
	npm --prefix frontend run dev > .dev-frontend.log 2>&1 & echo $$! > .dev-frontend.pid
	@sleep 1
	@echo ""
	@echo "  Backend   → http://localhost:8081"
	@echo "  Frontend  → http://localhost:5173  (proxies /api + /grafana to backend)"
	@echo "  MCP       → alertmanager :8000, grafana :8001, genesyscloud :8002"
	@echo ""
	@echo "  Logs:  make dev-logs"
	@echo "  Stop:  make dev-stop"

dev-stop: ## Stop all dev processes
	@$(MAKE) mcp-stop --no-print-directory
	@for name in backend frontend; do \
		if [ -f .dev-$$name.pid ]; then \
			kill $$(cat .dev-$$name.pid) 2>/dev/null; \
			rm -f .dev-$$name.pid; \
			echo "Stopped $$name"; \
		fi; \
	done

dev-restart: dev ## Rebuild and restart everything

dev-logs: ## Tail all logs
	@tail -f .dev-backend.log .dev-frontend.log .mcp-alertmanager.log .mcp-grafana.log .mcp-genesyscloud.log 2>/dev/null

frontend-build: ## Build frontend into static/ for embedding
	npm --prefix frontend run build

package: frontend-build build ## Build frontend + Go binary for single-file deploy
	@echo "Packaged binary: $(BIN)"

install-systemd: package ## Install systemd unit and binary (requires sudo)
	sudo install -m 0755 $(BIN) /opt/monitoring-assistant/assistant
	sudo install -m 0644 deploy/monitoring-assistant.service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo "Installed systemd unit. Edit /etc/monitoring-assistant/config.yaml as needed."

help: ## Show this help
	@grep -E '^[a-z][a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
