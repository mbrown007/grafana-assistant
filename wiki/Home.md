# Welcome to the Monitoring Assistant Wiki!

The Monitoring Assistant is a web application that wraps a Grafana instance, providing an LLM-powered chat assistant alongside the standard Grafana dashboards. The assistant can understand the context of the dashboard being viewed and can answer questions about panels, queries, and metrics. It can also interact with other monitoring tools like Prometheus and Alertmanager via a set of "Monitoring Control Plane" (MCP) servers.

<img width="1919" height="947" alt="image" src="https://github.com/user-attachments/assets/bfd16a18-0f8a-404e-b8a0-0b7a062662b6" />

## Key Features

*   **Grafana Integration:** Embeds Grafana dashboards in an iframe.
*   **Chat Assistant:** Provides an LLM-powered chat assistant that can answer questions about the current dashboard.
*   **Tool Integration:** Connects to MCP servers to fetch live data from various monitoring tools.
*   **Scratchpad Dashboards:** Can create temporary, per-session Grafana dashboards to visualize ad-hoc queries.
*   **Observability:** Emits Prometheus metrics and structured logs for its own operation.

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js 18+ (for frontend build)
- Docker (for the Grafana/Prometheus/Alertmanager dev-test stack)
- OpenAI API key

### 1. Start the observability stack

```bash
docker compose -f dev-test-docker-compose.yml up -d --wait
```

This starts:
- Grafana: http://localhost:13000/grafana (admin/admin, anonymous admin enabled)
- Prometheus: http://localhost:19090
- Alertmanager: http://localhost:19093
- Loki: http://localhost:13100

### 2. Configure the assistant

```bash
cp config.example.yaml config.yaml
```

Edit `config.yaml` — at minimum set `grafana_url` and `listen_addr`.

Put secrets in `.env`:

```bash
ASSISTANT_OPENAI_API_KEY=sk-...
ALERTMANAGER_URL=http://localhost:19093
GRAFANA_URL=http://localhost:13000
GRAFANA_USERNAME=admin
GRAFANA_PASSWORD=admin
```

### 3. Install frontend dependencies

```bash
npm --prefix frontend install
```

### 4. Start everything

```bash
make dev
```

Open http://localhost:5173 in your browser.
