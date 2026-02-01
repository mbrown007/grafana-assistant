# Deployment Guide

## Build and package

The production binary embeds the built frontend assets in `frontend/dist/`.

```bash
make package
```

This runs `make frontend-build` and then `go build`, producing `bin/assistant`.

## Production quickstart

Copy/paste sequence for a basic single-host deployment (adjust paths and user as needed):

```bash
sudo useradd --system --home /opt/monitoring-assistant --shell /usr/sbin/nologin monitoring-assistant
sudo mkdir -p /opt/monitoring-assistant /etc/monitoring-assistant /var/lib/monitoring-assistant
sudo chown -R monitoring-assistant:monitoring-assistant /opt/monitoring-assistant /var/lib/monitoring-assistant

make package
sudo install -m 0755 bin/assistant /opt/monitoring-assistant/assistant
sudo install -m 0640 config.example.yaml /etc/monitoring-assistant/config.yaml
sudo install -m 0640 deploy/assistant.env.example /etc/monitoring-assistant/assistant.env
sudo chmod 600 /etc/monitoring-assistant/assistant.env

sudo install -m 0644 deploy/monitoring-assistant.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now monitoring-assistant
```

## Recommended layout

```
/opt/monitoring-assistant/assistant
/etc/monitoring-assistant/config.yaml
/etc/monitoring-assistant/assistant.env
/var/lib/monitoring-assistant/assistant.db
```

Copy `config.example.yaml` to `/etc/monitoring-assistant/config.yaml` and adjust:

- `listen_addr`
- `grafana_url`
- `grafana_token`
- `db_path`
- `data_retention_days`
- `openai_api_key` / `openai_model`
- `mcp_servers`
- `kb_path` (if your KB is not in the working directory)
- `kb_max_sections` / `kb_max_section_chars` (if you want to tune KB context size)

Store secrets in `/etc/monitoring-assistant/assistant.env` and set permissions to `600`.

## systemd unit

Use the unit file in `deploy/monitoring-assistant.service` as a template.

```bash
sudo cp deploy/monitoring-assistant.service /etc/systemd/system/
sudo mkdir -p /etc/systemd/system/monitoring-assistant.service.d
sudo install -m 0644 deploy/monitoring-assistant.service.d/override.conf.example /etc/systemd/system/monitoring-assistant.service.d/override.conf
sudo systemctl daemon-reload
sudo systemctl enable --now monitoring-assistant
```

Adjust `User`, `Group`, `WorkingDirectory`, and paths in `ExecStart` as needed.
If you use MCP stdio mode, the MCP processes inherit the assistant's environment,
so put MCP secrets/vars in `/etc/monitoring-assistant/assistant.env` or a
drop-in `assistant.secrets` file referenced by the override.

## KB indexing (optional)

If you have a larger KB folder or want faster startup, build an index file:

```bash
make kb-reindex
```

This writes `KB/.kb_index.json`, which the assistant and KB MCP server will load
if present.

## Grafana configuration

Grafana must allow iframe embedding and subpath proxying. See `docs/GRAFANA_CONFIG.md`.

## Network and firewall

Minimum ports (adjust for your environment):

- Inbound: `listen_addr` (default `:8080`) for the wrapper UI and API
- Local outbound to Grafana (default `:3000`) for API + proxy
- Local outbound to MCP servers (default `:8000`, `:8001`, `:8002`) if enabled
- Outbound to OpenAI for LLM requests

If everything runs on the same host, bind Grafana and MCP servers to `127.0.0.1`
and only expose the wrapper port externally.

## Security posture

Current security model:

- Auth relies on Grafana session cookies (no separate auth layer).
- No external admin UI; the wrapper UI is the only surface area.
- SQLite is local and scoped to the host filesystem.

Recommended deployment controls:

- Run as a dedicated unprivileged user.
- Keep `/etc/monitoring-assistant` readable only by that user.
- Place the wrapper behind TLS (reverse proxy) and set `listen_addr` to `127.0.0.1`.
- Restrict outbound access to only Grafana, MCP servers, and OpenAI.

## Data retention

`data_retention_days` controls how long chat sessions remain in SQLite.
On startup and then every 24 hours, the assistant purges sessions older than
the configured retention window. Store the database on durable storage and
include it in backups if retention requirements demand it.
