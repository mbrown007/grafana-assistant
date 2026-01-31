# Monitoring Assistant

A Grafana wrapper that embeds dashboards in an iframe alongside an LLM-powered chat sidebar. The assistant understands which dashboard you're viewing and can answer questions about panels, queries, and metrics.

## Prerequisites

- Go 1.25+
- Node.js 18+ (for frontend build)
- Docker (for the Grafana/OTEL-LGTM stack)

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

### 3. Build the frontend

```bash
cd frontend
npm install
npm run build
cd ..
```

This outputs static assets to `cmd/assistant/static/`, which are embedded into the Go binary.

### 4. Run the server

```bash
go run ./cmd/assistant -config config.yaml
```

Open `http://localhost:8081/` — you should see Grafana in the main pane with a chat sidebar on the right.

## Development

For frontend development with hot reload, run the Go server and Vite dev server separately:

```bash
# Terminal 1 — Go backend
go run ./cmd/assistant -config config.yaml

# Terminal 2 — Vite dev server (proxies /api and /grafana to the Go backend)
cd frontend
npm run dev
```

Then open `http://localhost:5173/`. The Vite proxy is configured to forward `/api/*` and `/grafana/*` to `http://localhost:8081` (override with `VITE_DEV_PROXY_TARGET` env var).

## Project structure

```
cmd/assistant/          Go entrypoint + embedded static assets
internal/
  api/                  HTTP API types (chat request/response, stream chunks)
  auth/                 Session resolver (cookie → Grafana user)
  config/               YAML config loader with env var overrides
  context/              Dashboard URL parser + enricher with TTL cache
  grafana/              Grafana API client (dashboards, users)
  llm/                  OpenAI streaming client
  mcp/                  MCP server integration types
  proxy/                Grafana reverse proxy (header stripping, cookie passthrough)
  storage/              SQLite storage for chat history
frontend/               React + TypeScript + Vite
  src/
    components/         ChatPanel, Artifact renderer, Markdown renderer
    services/           SSE streaming API client
    utils/              Dashboard URL parsing
```

## Tests

```bash
go test ./...
```

## Grafana configuration notes

The assistant proxies Grafana under `/grafana/`. This requires Grafana to know about the subpath:

| Environment variable | Value | Purpose |
|---|---|---|
| `GF_SERVER_ROOT_URL` | `http://<host>:<port>/grafana/` | Tells Grafana its external URL |
| `GF_SERVER_SERVE_FROM_SUB_PATH` | `true` | Makes Grafana handle the `/grafana/` prefix |
| `GF_SECURITY_ALLOW_EMBEDDING` | `true` | Allows Grafana to be loaded in an iframe |

Without these, Grafana will fail to load its frontend assets when accessed through the proxy.
