# Architecture

The Monitoring Assistant is composed of several key components that work together to provide its functionality.

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

## Project Structure

The project is a monorepo containing the main Go backend, a React frontend, and several MCP servers.

*   `cmd/assistant/`: The main entrypoint for the Go backend.
*   `internal/`: Internal Go packages for the backend, organized by domain:
    *   `agent/`: Agent orchestration, memory, prompts, tool routing
    *   `api/`: HTTP API types and SSE streaming handler
    *   `auth/`: Session resolver (cookie -> Grafana user)
    *   `config/`: YAML config loader with env var overrides + .env support
    *   `context/`: Dashboard URL parser + enricher with TTL cache
    *   `grafana/`: Grafana API client (dashboards, users)
    *   `llm/`: OpenAI streaming client
    *   `mcp/`: MCP clients (SSE and stdio transports)
    *   `proxy/`: Grafana reverse proxy (header stripping, cookie passthrough)
    *   `storage/`: SQLite storage for chat history
*   `frontend/`: The React frontend application built with TypeScript and Vite.
    *   `src/components/`: ChatPanel, Artifact renderer, Markdown renderer
    *   `src/services/`: SSE streaming API client
    *   `src/utils/`: Dashboard URL parsing
*   `mcp_servers/`: A collection of Go-based MCP servers that provide tools for the assistant to interact with other monitoring systems.
    *   `alertmanager-mcp-go/`: AlertManager MCP server
    *   `mcp-grafana/`: Grafana MCP server
    *   `genesys-cloud-mcp-go/`: Genesys Cloud MCP server
*   `KB/`: Domain knowledge base (markdown sections).
*   `docs/`: Project documentation.
*   `deploy/`: Deployment-related files, such as systemd unit files and example configurations.
*   `containers/`: Docker-related files for setting up a local development environment.
*   `tests/`: Integration and end-to-end tests.

## Chat Flow

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
