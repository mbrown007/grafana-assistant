# Deployment Guide

This guide provides instructions for deploying the Monitoring Assistant to a production environment.

## Packaging the Application

For production, it's recommended to package the application into a single binary. This binary will include the Go backend and all the necessary frontend assets.

You can create this package by running:

```bash
make package
```

This command will first build the frontend assets (by running `make frontend-build`) and then build the Go binary, embedding the frontend assets into it. The final binary will be located at `bin/assistant`.

## Single-Host Deployment (stdio transport)

For single-host deployments where all binaries run together, you can use the `deploy-lab` Make command:

```bash
# Build all binaries and create deployment folder
make deploy-lab
```

This creates a `deploy/lab/` directory with the following structure:
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

You can then deploy this to your server:
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

With stdio transport, the assistant spawns MCP servers as child processes, so no separate services are needed.

## Manual Deployment with systemd

### Recommended Layout

```
/opt/monitoring-assistant/assistant
/etc/monitoring-assistant/config.yaml
/etc/monitoring-assistant/assistant.env
/var/lib/monitoring-assistant/assistant.db
```

### Setup

1.  **Create user and directories:**
    ```bash
    sudo useradd --system --home /opt/monitoring-assistant --shell /usr/sbin/nologin monitoring-assistant
    sudo mkdir -p /opt/monitoring-assistant /etc/monitoring-assistant /var/lib/monitoring-assistant
    sudo chown -R monitoring-assistant:monitoring-assistant /opt/monitoring-assistant /var/lib/monitoring-assistant
    ```

2.  **Install files:**
    ```bash
    make package
    sudo install -m 0755 bin/assistant /opt/monitoring-assistant/assistant
    sudo install -m 0640 config.example.yaml /etc/monitoring-assistant/config.yaml
    sudo install -m 0640 deploy/assistant.env.example /etc/monitoring-assistant/assistant.env
    sudo chmod 600 /etc/monitoring-assistant/assistant.env
    ```

3.  **Configure:**
    Copy `config.example.yaml` to `/etc/monitoring-assistant/config.yaml` and adjust settings as needed. Store secrets in `/etc/monitoring-assistant/assistant.env`.
    For safer rollout defaults, review and tune:
    - `model_profile` (prompt profile selection by model family)
    - `request_budget` (per-request token/tool/cost guardrails)
    - `feature_flags` (routing/sub-agent/composite/judge-gate/redaction toggles)

4.  **systemd service:**
    Use the unit file in `deploy/monitoring-assistant.service` as a template.
    ```bash
    sudo cp deploy/monitoring-assistant.service /etc/systemd/system/
    sudo mkdir -p /etc/systemd/system/monitoring-assistant.service.d
    sudo install -m 0644 deploy/monitoring-assistant.service.d/override.conf.example /etc/systemd/system/monitoring-assistant.service.d/override.conf
    sudo systemctl daemon-reload
    sudo systemctl enable --now monitoring-assistant
    ```

## HAProxy Configuration

When running behind HAProxy (e.g., Grafana at root, assistant at `/assistant`):

**Frontend** - Add routing rule:
```cfg
frontend https_front
    # ...
    use_backend assistant_backend if { path_beg /assistant }
    http-request redirect code 301 location /assistant/ if { path -i /assistant }
```

**Backend** - Strip prefix and forward headers:
```cfg
backend assistant_backend
    mode http
    http-request set-path %[path,regsub(^/assistant,)]
    http-request set-header X-Forwarded-Proto https
    http-request set-header X-Forwarded-Host %[req.hdr(Host)]
    http-request set-header X-Forwarded-Prefix /assistant
    server assistant 127.0.0.1:5480 check
```

**Assistant config** - Use empty `base_path`:
```yaml
listen_addr: ":5480"
base_path: ""  # Empty - HAProxy strips /assistant, X-Forwarded-Prefix tells us the real path
```

## Security

*   Run the application as a dedicated unprivileged user.
*   Restrict read access to `/etc/monitoring-assistant` to that user.
*   Place the wrapper behind a TLS-terminating reverse proxy.
*   Restrict outbound network access to only what is necessary (Grafana, MCP servers, OpenAI).

## Data Retention

The `data_retention_days` setting in `config.yaml` controls how long chat history is kept in the SQLite database. A cleanup job runs on startup and every 24 hours to purge old records.
