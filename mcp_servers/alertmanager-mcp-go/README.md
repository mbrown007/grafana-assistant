# Prometheus Alertmanager MCP Server (Go)

A Go implementation of the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server for Prometheus Alertmanager. This enables AI assistants and tools to query and manage Alertmanager resources programmatically and securely.

## Features

- Query Alertmanager status, alerts, silences, receivers, and alert groups
- **Smart pagination support** to prevent LLM context window overflow when handling large numbers of alerts
- Create, update, and delete silences
- Create new alerts
- Authentication support (Basic auth via environment variables)
- Multi-tenant support (via `X-Scope-OrgId` header for Mimir/Cortex)
- Multiple transport modes: stdio and SSE
- Docker containerization support
- Configurable pagination limits

## Prerequisites

- Go 1.23 or higher
- Docker (optional, for containerized deployment)
- Access to a Prometheus Alertmanager server

## Installation

### From Source

```bash
git clone https://github.com/your-org/alertmanager-mcp-go.git
cd alertmanager-mcp-go
make build
```

### Using Docker

```bash
docker build -t alertmanager-mcp-go .
```

## Configuration

Configure the server using environment variables. Copy `.env.sample` to `.env` and adjust the values:

```bash
cp .env.sample .env
```

### Required Configuration

- `ALERTMANAGER_URL`: URL of your Alertmanager instance (e.g., `http://localhost:9093`)

### Optional Configuration

- `ALERTMANAGER_USERNAME`: Basic auth username (optional)
- `ALERTMANAGER_PASSWORD`: Basic auth password (optional)
- `ALERTMANAGER_TENANT`: Static tenant ID for multi-tenant setups (optional)

### Transport Configuration

- `MCP_TRANSPORT`: Transport mode - `stdio` or `sse` (default: `stdio`)
- `MCP_HOST`: Host to bind to for SSE transport (default: `0.0.0.0`)
- `MCP_PORT`: Port to listen on for SSE transport (default: `8000`)

### Pagination Configuration

- `ALERTMANAGER_DEFAULT_SILENCE_PAGE`: Default page size for silences (default: `10`)
- `ALERTMANAGER_MAX_SILENCE_PAGE`: Maximum page size for silences (default: `50`)
- `ALERTMANAGER_DEFAULT_ALERT_PAGE`: Default page size for alerts (default: `10`)
- `ALERTMANAGER_MAX_ALERT_PAGE`: Maximum page size for alerts (default: `25`)
- `ALERTMANAGER_DEFAULT_ALERT_GROUP_PAGE`: Default page size for alert groups (default: `3`)
- `ALERTMANAGER_MAX_ALERT_GROUP_PAGE`: Maximum page size for alert groups (default: `5`)

## Usage

### Running Locally

#### Stdio Mode (Default)

```bash
make run
```

Or with environment variables:

```bash
ALERTMANAGER_URL=http://localhost:9093 ./alertmanager-mcp-server
```

#### SSE Mode

```bash
make run-sse
```

Or directly:

```bash
./alertmanager-mcp-server -transport sse -host 0.0.0.0 -port 8000
```

### Running with Docker

#### Stdio Mode

```bash
docker run --rm -i \
  -e ALERTMANAGER_URL=http://your-alertmanager:9093 \
  -e ALERTMANAGER_USERNAME=your_username \
  -e ALERTMANAGER_PASSWORD=your_password \
  alertmanager-mcp-go
```

#### SSE Mode

```bash
docker run --rm \
  -e ALERTMANAGER_URL=http://your-alertmanager:9093 \
  -e ALERTMANAGER_USERNAME=your_username \
  -e ALERTMANAGER_PASSWORD=your_password \
  -e MCP_TRANSPORT=sse \
  -p 8000:8000 \
  alertmanager-mcp-go
```

### Using with Claude Desktop

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

#### Stdio Transport

```json
{
  "mcpServers": {
    "alertmanager": {
      "command": "/path/to/alertmanager-mcp-server",
      "env": {
        "ALERTMANAGER_URL": "http://your-alertmanager:9093",
        "ALERTMANAGER_USERNAME": "your_username",
        "ALERTMANAGER_PASSWORD": "your_password"
      }
    }
  }
}
```

#### Using Docker

```json
{
  "mcpServers": {
    "alertmanager": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "-e",
        "ALERTMANAGER_URL",
        "-e",
        "ALERTMANAGER_USERNAME",
        "-e",
        "ALERTMANAGER_PASSWORD",
        "alertmanager-mcp-go:latest"
      ],
      "env": {
        "ALERTMANAGER_URL": "http://your-alertmanager:9093",
        "ALERTMANAGER_USERNAME": "your_username",
        "ALERTMANAGER_PASSWORD": "your_password"
      }
    }
  }
}
```

## Multi-tenant Support

For multi-tenant Alertmanager deployments (e.g., Grafana Mimir, Cortex), you can specify the tenant ID using the `ALERTMANAGER_TENANT` environment variable for static configuration.

**Note:** The current version of the mcp-go library does not support extracting custom headers from individual requests. Unlike the Python implementation, dynamic per-request tenant switching via the `X-Scope-OrgId` header is not yet supported. The tenant ID from the environment variable will be used for all requests.

## Available Tools

The MCP server exposes the following tools, following [Alertmanager API v2](https://github.com/prometheus/alertmanager/blob/main/api/v2/openapi.yaml):

### get_status

Get current status of an Alertmanager instance and its cluster.

**Parameters:** None

### get_alerts

Get a list of alerts currently in Alertmanager.

**Parameters:**
- `filter` (string, optional): Filtering query (e.g., `alertname=~'.*CPU.*'`)
- `silenced` (boolean, optional): Include silenced alerts
- `inhibited` (boolean, optional): Include inhibited alerts
- `active` (boolean, optional): Include active alerts
- `count` (integer, optional): Number of alerts per page (default: 10, max: 25)
- `offset` (integer, optional): Number of alerts to skip (default: 0)

**Returns:** Paginated alert list with metadata

### get_alert_groups

Get a list of alert groups.

**Parameters:**
- `silenced` (boolean, optional): Include silenced alerts
- `inhibited` (boolean, optional): Include inhibited alerts
- `active` (boolean, optional): Include active alerts
- `count` (integer, optional): Number of alert groups per page (default: 3, max: 5)
- `offset` (integer, optional): Number of alert groups to skip (default: 0)

**Returns:** Paginated alert groups with metadata

### get_silences

Get list of all silences.

**Parameters:**
- `filter` (string, optional): Filtering query (e.g., `alertname=~'.*CPU.*'`)
- `count` (integer, optional): Number of silences per page (default: 10, max: 50)
- `offset` (integer, optional): Number of silences to skip (default: 0)

**Returns:** Paginated silence list with metadata

### post_silence

Create a new silence or update an existing one.

**Parameters:**
- `silence` (object, required): Silence object with the following fields:
  - `matchers` (array): List of matchers to match alerts
  - `startsAt` (string): Start time (RFC3339 format)
  - `endsAt` (string): End time (RFC3339 format)
  - `createdBy` (string): Name of the user creating the silence
  - `comment` (string): Comment for the silence

**Returns:** Created silence ID

### delete_silence

Delete a silence by its ID.

**Parameters:**
- `silence_id` (string, required): ID of the silence to delete

**Returns:** Success message

### post_alerts

Create new alerts.

**Parameters:**
- `alerts` (array, required): List of alert objects with:
  - `startsAt` (string): Start time (RFC3339 format)
  - `endsAt` (string, optional): End time (RFC3339 format)
  - `annotations` (object, optional): Alert annotations
  - `labels` (object, optional): Alert labels

**Returns:** Success message

### get_receivers

Get list of all receivers (notification integrations).

**Parameters:** None

**Returns:** List of receiver names

## Pagination Benefits

When working with environments that have many alerts, silences, or alert groups, the pagination feature helps:

- **Prevent context overflow**: By default, only a limited number of items are returned per request
- **Efficient browsing**: LLMs can iterate through results using `offset` and `count` parameters
- **Smart limits**: Maximum limits prevent excessive context usage
- **Clear navigation**: `has_more` flag indicates when additional pages are available

Example pagination response:

```json
{
  "data": [...],
  "pagination": {
    "total": 100,
    "offset": 0,
    "count": 10,
    "requested_count": 10,
    "has_more": true
  }
}
```

## Development

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Code Formatting

```bash
make fmt
```

### Linting

```bash
make lint
```

Note: Requires [golangci-lint](https://golangci-lint.run/) to be installed.

### Docker Commands

```bash
# Build Docker image
make docker-build

# Run with Docker (stdio)
make docker-run

# Run with Docker (SSE)
make docker-run-sse

# Run with Docker (HTTP)
make docker-run-http
```

## Project Structure

```
alertmanager-mcp-go/
├── cmd/
│   └── server/
│       └── main.go          # Entry point
├── pkg/
│   ├── alertmanager/
│   │   └── client.go        # Alertmanager HTTP client
│   └── server/
│       ├── handlers.go      # MCP tool handlers
│       └── pagination.go    # Pagination utilities
├── Dockerfile               # Docker build configuration
├── Makefile                 # Build and run commands
├── .env.sample              # Example environment variables
├── go.mod                   # Go module definition
└── README.md                # This file
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

Apache 2.0

## Acknowledgments

This is a Go port of the [Python Alertmanager MCP Server](https://github.com/ntk148v/alertmanager-mcp-server) by @ntk148v.
