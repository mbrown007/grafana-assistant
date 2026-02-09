# Monitoring Assistant

A Grafana wrapper that embeds dashboards in an iframe alongside an LLM-powered chat sidebar. The assistant understands which dashboard you're viewing and can answer questions about panels, queries, and metrics. It connects to MCP tool servers to fetch live data from Alertmanager, Grafana APIs, and Genesys Cloud.
<img width="1919" height="947" alt="image" src="https://github.com/user-attachments/assets/bfd16a18-0f8a-404e-b8a0-0b7a062662b6" />

<img width="1919" height="947" alt="image" src="https://github.com/user-attachments/assets/20f07f85-96cb-4b6e-afe2-28383a4404c0" />


## Prerequisites

- Go 1.25+
- Node.js 18+ (for frontend build)
- Docker (for the Grafana/Prometheus/Alertmanager dev-test stack)
- OpenAI API key

## Quick start

### 1. Start the observability stack

```bash
docker compose -f dev-test-docker-compose.yml up -d --wait
```

This starts:
- Grafana: http://localhost:13000/grafana (admin/admin, anonymous admin enabled)
- Prometheus: http://localhost:19090
- Alertmanager: http://localhost:19093
- Loki: http://localhost:13100

Prometheus is configured to scrape the assistant at `http://host.docker.internal:8081/metrics`.
Promtail tails the audit log at `/var/log/grafana-assistant/*.log` (see `audit_log_path` in config).
If you don’t have permission to write there, set `audit_log_path` to a writable location.
You can also run `make audit-log-dir` to create the directory and log file.

### 2. Configure the assistant

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` — at minimum set `grafana_url` and `listen_addr`. The defaults expect Grafana at `http://localhost:13000` (served from `/grafana`) and the assistant on `:8081`.

Put secrets in `.env` (auto-loaded at startup, gitignored):

```bash
ASSISTANT_OPENAI_API_KEY=sk-...
ALERTMANAGER_URL=http://localhost:19093
GRAFANA_URL=http://localhost:13000
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
| `make build-all` | Build assistant + all MCP server binaries |
| `make frontend-build` | Build frontend into `frontend/dist/` for Go embedding |
| `make package` | Build frontend + Go binary for single-file deploy |
| `make mcp-build` | Build all MCP server binaries to `bin/` |
| `make deploy-lab` | Create `deploy/lab/` folder ready for deployment |

### MCP Servers

| Command | Description |
|---|---|
| `make mcp-start` | Build and start all MCP servers in SSE mode (background) |
| `make mcp-stop` | Stop all MCP servers |
| `make kb-reindex` | Build KB index from markdown files |

The three MCP servers and their default ports:

| Server | Port | Tools |
|---|---|---|
| Alertmanager | 8000 | Alerts, silences, receivers |
| Grafana | 8001 | Dashboards, datasources, Prometheus queries, Loki logs |
| Genesys Cloud | 8002 | Queue volumes, conversations, OAuth clients |
| KB | 8003 | KB search and section retrieval |

By default, `make mcp-start` launches Grafana MCP with a read-focused tool profile:
- Enabled categories: `search,datasource,prometheus,loki,alerting,dashboard,navigation`
- Disabled by default: write tools (`--disable-write`) and admin tools (`--disable-admin`)

Override for local experimentation:
```bash
GRAFANA_MCP_ENABLED_TOOLS="search,datasource,prometheus,loki,alerting,dashboard,navigation,rendering" \
GRAFANA_MCP_EXTRA_FLAGS="--disable-write=false --disable-admin=false" \
make mcp-start
```

Per-server tool filtering is also available in assistant config (`mcp_servers` entries):
- `tool_allowlist`: only these tool names are exposed to the model
- `tool_denylist`: these tool names are always blocked (takes precedence)

Tool names can be provided as either full (`grafana__query_prometheus`) or short (`query_prometheus`) form.

Servers that fail to connect at startup are skipped -- the assistant still works without them.

If you want the assistant to spawn MCP servers via stdio (recommended for single-host installs),
use the stdio example config in `deploy/config.stdio.example.yaml` and ensure the assistant
service user can execute the MCP binaries. stdio servers inherit the assistant's environment.

The assistant can also load domain knowledge from a local KB folder (default: `KB/`).
For Genesys Cloud-focused deployments, keep your runbooks/metric references in that folder.
Run `make kb-reindex` after updating KB content to refresh the index (optional but faster).

## Deployment

For production packaging, run:

```bash
make package
```

This embeds the built frontend into the single `bin/assistant` binary. See
`docs/DEPLOYMENT.md` for a systemd unit template, recommended layout, and
network/security notes.

### Lab/Production Deployment (stdio transport)

For single-host deployments where all binaries run together:

```bash
# Build all binaries and create deployment folder
make deploy-lab
```

This creates `deploy/lab/` with:
```
deploy/lab/
├── assistant          # Main binary
├── mcp-grafana        # Grafana MCP server
├── mcp-alertmanager   # Alertmanager MCP server
├── mcp-kb             # Knowledge base MCP server
├── config.yaml        # Configuration (stdio transport)
├── run.sh             # Runner script
├── .env.example       # Environment template
└── KB/                # Knowledge base
```

Deploy to server:
```bash
scp -r deploy/lab/* user@server:~/grafana-assistant/
```

On the server:
```bash
cd ~/grafana-assistant
cp .env.example .env
nano .env              # Add ASSISTANT_OPENAI_API_KEY, ASSISTANT_GRAFANA_TOKEN

./run.sh               # Run in foreground (Ctrl+C to stop)
./run.sh start         # Run in background
./run.sh status        # Check status
./run.sh logs          # Tail logs
./run.sh stop          # Stop
```

With stdio transport, the assistant spawns MCP servers as child processes - no separate services needed.

### HAProxy Configuration

When running behind HAProxy (e.g., Grafana at root, assistant at `/assistant`):

**Frontend** - Add routing rule:
```cfg
frontend https_front
    bind *:443 ssl crt /etc/haproxy/certs/your-cert.pem
    mode http
    default_backend grafana_backend

    # Route /assistant/* to assistant backend
    use_backend assistant_backend if { path_beg /assistant }

    # Redirect /assistant to /assistant/ for consistent URLs
    http-request redirect code 301 location /assistant/ if { path -i /assistant }
```

**Backend** - Strip prefix and forward headers:
```cfg
backend assistant_backend
    mode http

    # Strip /assistant prefix (assistant receives /api/chat, not /assistant/api/chat)
    http-request set-path %[path,regsub(^/assistant,)]

    # CRITICAL: Tell assistant its external path for cookies and frontend config
    http-request set-header X-Forwarded-Proto https
    http-request set-header X-Forwarded-Host %[req.hdr(Host)]
    http-request set-header X-Forwarded-Prefix /assistant

    server assistant 127.0.0.1:5480 check
```

**Assistant config** - Use empty `base_path` (HAProxy strips the prefix):
```yaml
listen_addr: ":5480"
base_path: ""  # Empty - HAProxy strips /assistant, X-Forwarded-Prefix tells us the real path
allowed_origin: "https://your-domain.com"
```

The assistant reads `X-Forwarded-Prefix` to:
- Set correct cookie paths (`/assistant/`)
- Configure frontend base URL for assets and API calls

See `dev/haproxy/` for a complete Docker-based example setup.

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
                    +-- KB MCP (SSE :8003 or stdio)
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
KB/                     Domain knowledge base (markdown sections)
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
