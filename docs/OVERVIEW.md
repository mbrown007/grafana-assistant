# Project Overview

The Monitoring Assistant is a web application that wraps a Grafana instance, providing an LLM-powered chat assistant alongside the standard Grafana dashboards. The assistant can understand the context of the dashboard being viewed and can answer questions about panels, queries, and metrics. It can also interact with other monitoring tools like Prometheus and Alertmanager via a set of "Monitoring Control Plane" (MCP) servers.

## Key Features

*   **Grafana Integration:** Embeds Grafana dashboards in an iframe.
*   **Chat Assistant:** Provides an LLM-powered chat assistant that can answer questions about the current dashboard.
*   **Tool Integration:** Connects to MCP servers to fetch live data from various monitoring tools.
*   **Scratchpad Dashboards:** Can create temporary, per-session Grafana dashboards to visualize ad-hoc queries.
*   **Observability:** Emits Prometheus metrics and structured logs for its own operation.

## Project Structure

The project is a monorepo containing the main Go backend, a React frontend, and several MCP servers.

*   `cmd/assistant/`: The main entrypoint for the Go backend.
*   `internal/`: Internal Go packages for the backend, organized by domain (e.g., `agent`, `api`, `grafana`).
*   `frontend/`: The React frontend application.
*   `mcp_servers/`: A collection of Go-based MCP servers that provide tools for the assistant to interact with other monitoring systems.
*   `docs/`: Project documentation.
*   `deploy/`: Deployment-related files, such as systemd unit files and example configurations.
*   `containers/`: Docker-related files for setting up a local development environment.
*   `tests/`: Integration and end-to-end tests.

## Key Methods and Data Flows

### Chat Flow

The main data flow in the application is the chat flow:

1.  **User sends a message:** The user types a message in the chat panel and clicks "send".
2.  **Frontend sends request to backend:** The frontend sends a `POST` request to the `/api/chat` endpoint on the backend.
3.  **Backend enriches context:** The backend receives the request and enriches it with context about the current dashboard (if any).
4.  **Agent loop starts:** The `agent` package orchestrates the chat flow. It builds a system prompt with the user's message, the dashboard context, and the available tools.
5.  **LLM is called:** The agent calls the LLM (e.g., OpenAI) with the prompt.
6.  **Tool calls (if any):** If the LLM requests a tool call, the agent executes the tool via the appropriate MCP client. The result of the tool call is then sent back to the LLM.
7.  **LLM generates response:** The LLM generates a response, which is streamed back to the frontend via a Server-Sent Events (SSE) connection.
8.  **Frontend displays response:** The frontend receives the SSE chunks and displays the response in the chat panel.
9.  **History is persisted:** The conversation is persisted to the SQLite database.

## Further Reading

*   **Deployment:** For instructions on how to deploy the application, see [`DEPLOYMENT.md`](./DEPLOYMENT.md).
*   **Grafana Configuration:** For notes on how to configure Grafana for use with the assistant, see [`GRAFANA_CONFIG.md`](./GRAFANA_CONFIG.md).
*   **Observability:** For details on the metrics and logs emitted by the application, see [`OBSERVABILITY.md`](./OBSERVABILITY.md).
*   **Artifact Schema:** For details on the format of the artifacts that can be rendered in the chat, see [`ARTIFACT_SCHEMA.md`](./ARTIFACT_SCHEMA.md).
*   **Chat Flow:** For a detailed sequence diagram of the chat flow, see [`CHAT_FLOW.md`](./CHAT_FLOW.md).
