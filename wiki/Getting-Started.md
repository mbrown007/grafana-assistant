# Getting Started

This guide will walk you through setting up your local development environment for the Monitoring Assistant.

## Prerequisites

- Go 1.25+
- Node.js 18+ (for frontend build)
- Docker (for the Grafana/Prometheus/Alertmanager dev-test stack)
- OpenAI API key

## 1. Start the observability stack

The project includes a Docker Compose file to easily stand up the necessary services for local development.

```bash
docker compose -f dev-test-docker-compose.yml up -d --wait
```

This command will start the following services:

- **Grafana:** Available at `http://localhost:13000/grafana`. You can log in with the username `admin` and password `admin`. Anonymous admin access is also enabled.
- **Prometheus:** Available at `http://localhost:19090`.
- **Alertmanager:** Available at `http://localhost:19093`.
- **Loki:** Available at `http://localhost:13100`.

Prometheus is pre-configured to scrape the assistant's metrics endpoint at `http://host.docker.internal:8081/metrics`.
Promtail is set up to tail the audit log at `/var/log/grafana-assistant/*.log` (as defined by `audit_log_path` in the configuration). If you encounter permission issues for this path, you can either change `audit_log_path` to a writable location or run `make audit-log-dir` to create the directory and log file.

## 2. Configure the assistant

First, copy the example configuration file:

```bash
cp config.example.yaml config.yaml
```

Next, you'll need to edit `config.yaml`. At a minimum, you should set the `grafana_url` and `listen_addr`. The default configuration assumes Grafana is running at `http://localhost:13000` (and served from the `/grafana` sub-path) and the assistant will listen on port `:8081`.

Sensitive information, such as API keys, should be stored in a `.env` file. This file is loaded automatically at startup and is ignored by Git.

Create a `.env` file with the following content:

```bash
ASSISTANT_OPENAI_API_KEY=sk-...
ALERTMANAGER_URL=http://localhost:19093
GRAFANA_URL=http://localhost:13000
GRAFANA_USERNAME=admin
GRAFANA_PASSWORD=admin
```

## 3. Install frontend dependencies

If this is your first time setting up the project, you'll need to install the frontend dependencies:

```bash
npm --prefix frontend install
```

## 4. Start the application

You can start all the components of the Monitoring Assistant (MCP servers, Go backend, and Vite frontend dev server) with a single command:

```bash
make dev
```

Once everything is up and running, you can access the Monitoring Assistant in your browser at `http://localhost:5173`.

## Useful Make Commands

The project includes a `Makefile` with several useful commands for development:

### Development

| Command | Description |
|---|---|
| `make dev` | Build and start MCP servers, the Go backend, and the Vite frontend dev server. |
| `make dev-stop` | Stop all development processes. |
| `make dev-restart` | Rebuild and restart all components. |
| `make dev-logs` | Tail the log files for the backend, frontend, and MCP servers. |

### Building

| Command | Description |
|---|---|
| `make build` | Build the Go backend binary, which will be located at `bin/assistant`. |
| `make build-all` | Build the assistant and all MCP server binaries. |
| `make frontend-build` | Build the frontend into the `frontend/dist/` directory for embedding in the Go binary. |
| `make package` | Build the frontend and the Go binary for a single-file deployment. |
| `make mcp-build` | Build all MCP server binaries to the `bin/` directory. |
| `make deploy-lab` | Create a `deploy/lab/` folder that is ready for deployment. |
