# Backend

The Monitoring Assistant's backend is a Go application that serves the frontend, handles the API requests, and orchestrates the chat assistant's logic.

## Technologies

*   **Language:** Go
*   **Web Framework:** Standard library `net/http`
*   **Database:** SQLite for chat history and feedback storage
*   **LLM Integration:** OpenAI API

## Project Structure

The backend code is primarily located in the `cmd/assistant/` and `internal/` directories.

```
cmd/assistant/
├── main.go          # Application entry point
└── static/          # Embedded frontend assets

internal/
├── agent/           # Core agent logic, including prompts, memory, and tool routing
├── api/             # HTTP API handlers and types
├── auth/            # User authentication and session management
├── config/          # Configuration loading and management
├── context/         # Dashboard context parsing and enrichment
├── grafana/         # Grafana API client
├── kb/              # Knowledge Base integration
├── llm/             # LLM client (OpenAI)
├── mcp/             # Monitoring Control Plane client
├── middleware/      # HTTP middleware (e.g., logging, rate limiting)
├── proxy/           # Reverse proxy for Grafana
├── storage/         # Database interaction (SQLite)
└── ...
```

### Key Directories

*   **`cmd/assistant/`**: This is the main entry point for the application. The `main.go` file initializes the application, sets up the routes, and starts the HTTP server. The `static/` directory is used to embed the frontend assets into the Go binary for a single-file deployment.
*   **`internal/agent/`**: This is the heart of the chat assistant. It contains the logic for orchestrating the chat flow, managing the conversation history (memory), constructing prompts for the LLM, and routing tool calls to the appropriate MCP servers.
*   **`internal/api/`**: This directory contains the HTTP handlers for the API endpoints, as well as the data types used in the API.
*   **`internal/auth/`**: Handles user authentication by resolving the Grafana session cookie.
*   **`internal/config/`**: Manages the application's configuration, loading it from a YAML file and overriding with environment variables.
*   **`internal/grafana/`**: Provides a client for interacting with the Grafana API.
*   **`internal/kb/`**: Contains the logic for integrating with the Knowledge Base.
*   **`internal/llm/`**: This directory contains the client for interacting with the OpenAI API.
*   **`internal/mcp/`**: This directory contains the client for communicating with the MCP servers.
*   **`internal/proxy/`**: Implements the reverse proxy that forwards requests to the Grafana instance.
*   **`internal/storage/`**: This directory contains the code for interacting with the SQLite database, where chat history and user feedback are stored.

## Development

To build and run the backend, use the following `make` command:
```bash
make run
```
For a full development environment that includes the frontend and MCP servers, use:
```bash
make dev
```

## Building

To build the backend binary, run:
```bash
make build
```
This will create the `assistant` binary in the `bin/` directory. For a full production package that embeds the frontend, run `make package`.
