# Lab Deployment

## Files to Copy

Copy these to `/home/mbrown/grafana-assistant/` on the lab server:

```
/home/mbrown/grafana-assistant/
├── assistant          # Main binary
├── mcp-grafana        # Grafana MCP server
├── mcp-alertmanager   # Alertmanager MCP server
├── mcp-kb             # Knowledge base MCP server
├── config.yaml        # Configuration
├── run.sh             # Runner script
├── .env               # Environment variables (copy from .env.example)
├── data/              # Created automatically
│   ├── assistant.db   # SQLite database
│   └── audit.log      # Audit log
└── KB/                # Knowledge base (optional)
```

## Build All Binaries

From the project root:

```bash
# Build all binaries
make build-all

# Or build individually:
go build -o assistant ./cmd/assistant
go build -o mcp-grafana ./mcp_servers/mcp-grafana/cmd/mcp-grafana
go build -o mcp-alertmanager ./mcp_servers/alertmanager-mcp-go/cmd/server
go build -o mcp-kb ./mcp_servers/kb-mcp-go/cmd/server
```

## Deploy to Lab

```bash
# Copy all files
scp assistant mcp-grafana mcp-alertmanager mcp-kb \
    dev/lab/config.yaml dev/lab/run.sh dev/lab/.env.example \
    mbrown@10.222.251.176:~/grafana-assistant/

# Copy KB if needed
scp -r KB mbrown@10.222.251.176:~/grafana-assistant/
```

## Setup on Lab Server

```bash
cd ~/grafana-assistant

# Create .env from example
cp .env.example .env
nano .env  # Add your API keys

# Make scripts executable
chmod +x run.sh assistant mcp-*

# Create data directory
mkdir -p data
```

## Environment Variables (.env)

```bash
# Required
ASSISTANT_OPENAI_API_KEY=sk-...
ASSISTANT_GRAFANA_TOKEN=glsa_...

# Optional - passed to MCP servers
GRAFANA_URL=http://127.0.0.1:3000
ALERTMANAGER_URL=http://127.0.0.1:9093
```

## HAProxy Configuration

Update `/etc/haproxy/haproxy.cfg`:

```cfg
backend assistant
  mode http
  http-request set-path %[path,regsub(^/assistant,)]
  http-request set-header X-Forwarded-Prefix /assistant
  http-request set-header X-Forwarded-Proto https
  server monitoring-rocky 127.0.0.1:5480 check
```

Then reload: `sudo systemctl reload haproxy`

## Usage

```bash
cd ~/grafana-assistant

# Run in foreground (Ctrl+C to stop)
./run.sh

# Or run in background
./run.sh start
./run.sh status
./run.sh logs
./run.sh stop
./run.sh restart
```

## Access

https://mon.sabio.cloud/assistant/

## Troubleshooting

Check logs:
```bash
./run.sh logs
# or
tail -f assistant.log
```

Verify MCP servers are discovered:
```bash
curl -s http://localhost:5480/healthz
```
