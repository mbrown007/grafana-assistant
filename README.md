# Monitoring Assistant

A Grafana wrapper that embeds dashboards in an iframe alongside an LLM-powered chat sidebar. The assistant understands which dashboard you're viewing and can answer questions about panels, queries, and metrics. It connects to MCP tool servers to fetch live data from Alertmanager, Grafana APIs, and Genesys Cloud.

## Prerequisites

- Go 1.25+
- Node.js 18+ (for frontend build)
- Docker (for the Grafana/OTEL-LGTM stack)
- OpenAI API key

## Quick start

### 1. Start the observability stack

```bash
cd /path/to/docker-otel-lgtm
```

Add the following to `.env` (required for subpath proxying and iframe embedding):

```
GF_SERVER_ROOT_URL=http://localhost:8081/grafana/
GF_SERVER_SERVE_FROM_SUB_PATH=true
GF_AUTH_ANONYMOUS_ENABLED=true
GF_SECURITY_ALLOW_EMBEDDING=true
```

Then start the container:

```bash
./run-lgtm.sh
```

### 2. Configure the assistant

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` — at minimum set `grafana_url` and `listen_addr`. The defaults expect Grafana at `http://localhost:3000` and the assistant on `:8081`.

Put secrets in `.env` (auto-loaded at startup, gitignored):

```bash
ASSISTANT_OPENAI_API_KEY=sk-...
ALERTMANAGER_URL=http://localhost:9093
GRAFANA_URL=http://localhost:3000
GRAFANA_USERNAME=admin
GRAFANA_PASSWORD=admin
```

### 3. Install frontend dependencies (first time only)

```bash
npm --prefix frontend install
```

### 4. Start everything

```bash
make dev
```

This builds and starts the MCP servers, Go backend, and Vite frontend dev server. Open http://localhost:5173 in your browser.

## Make Commands

### Development

| Command | Description |
|---|---|
| `make dev` | Build and start MCP servers + Go backend + Vite frontend |
| `make dev-stop` | Stop all dev processes |
| `make dev-restart` | Rebuild and restart everything |
| `make dev-logs` | Tail all log files (backend, frontend, MCP servers) |

### Build

| Command | Description |
|---|---|
| `make build` | Build the Go backend binary to `bin/assistant` |
| `make frontend-build` | Build frontend into `static/` for Go embedding |
| `make package` | Build frontend + Go binary for single-file deploy |
| `make mcp-build` | Build all MCP server binaries to `bin/` |

### MCP Servers

| Command | Description |
|---|---|
| `make mcp-start` | Build and start all MCP servers in SSE mode (background) |
| `make mcp-stop` | Stop all MCP servers |

The three MCP servers and their default ports:

| Server | Port | Tools |
|---|---|---|
| Alertmanager | 8000 | Alerts, silences, receivers |
| Grafana | 8001 | Dashboards, datasources, Prometheus queries, Loki logs |
| Genesys Cloud | 8002 | Queue volumes, conversations, OAuth clients |

Servers that fail to connect at startup are skipped -- the assistant still works without them.

If you want the assistant to spawn MCP servers via stdio (recommended for single-host installs),
use the stdio example config in `deploy/config.stdio.example.yaml` and ensure the assistant
service user can execute the MCP binaries. stdio servers inherit the assistant's environment.

## Deployment

For production packaging, run:

```bash
make package
```

This embeds the built frontend into the single `bin/assistant` binary. See
`docs/DEPLOYMENT.md` for a systemd unit template, recommended layout, and
network/security notes.

### Other

| Command | Description |
|---|---|
| `make run` | Build and run the backend in foreground |
| `make test` | Run all Go tests |
| `make lint` | Run golangci-lint |
| `make clean` | Remove build artifacts |
| `make help` | Show all available commands |

## Architecture

```
Browser (localhost:5173)
  |
  +-- Vite dev server (proxies /api, /grafana to backend)
        |
        +-- Go backend (localhost:8081)
              |
              +-- /grafana/*     --> reverse proxy to Grafana
              +-- /api/chat      --> SSE streaming chat (agent loop)
              +-- /api/dashboard-context/{uid}
              |
              +-- MCP clients (SSE or stdio)
                    +-- Alertmanager MCP (SSE :8000 or stdio)
                    +-- Grafana MCP (SSE :8001 or stdio)
                    +-- Genesys Cloud MCP (SSE :8002 or stdio)
```

The chat agent loop:
1. Enriches dashboard context server-side (panels, queries)
2. Builds a system prompt with context + available tools
3. Calls the LLM (OpenAI) with streaming
4. If the LLM requests tool calls, executes them via MCP and loops (max 5 iterations)
5. Streams the final response as SSE chunks
6. Persists conversation to SQLite

## Project structure

```
cmd/assistant/          Go entrypoint + embedded static assets
internal/
  agent/                Agent orchestration, memory, prompts, tool routing
  api/                  HTTP API types and SSE streaming handler
  auth/                 Session resolver (cookie -> Grafana user)
  config/               YAML config loader with env var overrides + .env support
  context/              Dashboard URL parser + enricher with TTL cache
  grafana/              Grafana API client (dashboards, users)
  llm/                  OpenAI streaming client
  mcp/                  MCP clients (SSE and stdio transports)
  proxy/                Grafana reverse proxy (header stripping, cookie passthrough)
  storage/              SQLite storage for chat history
frontend/               React + TypeScript + Vite
  src/
    components/         ChatPanel, Artifact renderer, Markdown renderer
    services/           SSE streaming API client
    utils/              Dashboard URL parsing
mcp_servers/
  alertmanager-mcp-go/  AlertManager MCP server
  mcp-grafana/          Grafana MCP server
  genesys-cloud-mcp-go/ Genesys Cloud MCP server
```

## Tests

```bash
make test
```

## Grafana configuration notes

The assistant proxies Grafana under `/grafana/`. This requires Grafana to know about the subpath:

| Environment variable | Value | Purpose |
|---|---|---|
| `GF_SERVER_ROOT_URL` | `http://<host>:<port>/grafana/` | Tells Grafana its external URL |
| `GF_SERVER_SERVE_FROM_SUB_PATH` | `true` | Makes Grafana handle the `/grafana/` prefix |
| `GF_SECURITY_ALLOW_EMBEDDING` | `true` | Allows Grafana to be loaded in an iframe |

Without these, Grafana will fail to load its frontend assets when accessed through the proxy.
