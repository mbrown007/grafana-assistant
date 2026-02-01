# Genesys Cloud MCP Server (Go)

A Model Context Protocol (MCP) server for Genesys Cloud Platform API, written in Go.

## Features

- ✅ **Search Queues** - Find queues by name with wildcard support
- ✅ **Query Queue Volumes** - Get conversation volumes for queues
- ✅ **Sample Conversations** - Retrieve representative conversation samples
- ✅ **Search Voice Conversations** - Search conversations with filters
- ✅ **OAuth Clients** - List and manage OAuth clients
- 🔄 **Stdio & SSE Transports** - Flexible integration options
- 🔐 **OAuth 2.0 Client Credentials** - Secure authentication
- 🌍 **Multi-Region Support** - Works with all Genesys Cloud regions

## Quick Start

### Prerequisites

- Go 1.21 or later
- Genesys Cloud OAuth Client credentials
- Access to Genesys Cloud organization

### Installation

1. Clone the repository:
```bash
git clone https://github.com/sabio/genesys-cloud-mcp-go.git
cd genesys-cloud-mcp-go
```

2. Install dependencies:
```bash
make deps
```

3. Configure environment variables:
```bash
cp .env.sample .env
# Edit .env with your credentials
```

4. Build:
```bash
make build
```

### Configuration

Create a `.env` file with the following:

```env
# Genesys Cloud Configuration
GENESYSCLOUD_REGION=mypurecloud.com           # Your Genesys region
GENESYSCLOUD_OAUTHCLIENT_ID=your-client-id
GENESYSCLOUD_OAUTHCLIENT_SECRET=your-secret

# MCP Server Configuration
MCP_TRANSPORT=stdio    # Options: stdio, sse
MCP_HOST=0.0.0.0      # For SSE transport
MCP_PORT=8080          # For SSE transport
```

**Supported Regions:**
- `mypurecloud.com` (Americas)
- `mypurecloud.ie` (EMEA)
- `mypurecloud.de` (Germany)
- `mypurecloud.jp` (Japan)
- `mypurecloud.com.au` (Australia)

### Running

**Stdio Mode** (default):
```bash
make run
```

**SSE Mode**:
```bash
make run-sse
```

**Docker**:
```bash
# Build image
make docker-build

# Run (stdio)
make docker-run

# Run (SSE)
make docker-run-sse
```

## Tools

### 1. search_queues

Search for routing queues by name.

**Parameters:**
- `name` (string, optional): Queue name to search
- `pageNumber` (number, optional): Page number (default: 1)
- `pageSize` (number, optional): Page size (default: 25, max: 100)

**Example:**
```json
{
  "name": "Support",
  "pageNumber": 1,
  "pageSize": 25
}
```

### 2. query_queue_volumes

Get conversation volumes for specified queues.

**Parameters:**
- `queueIds` (array, required): List of queue IDs
- `startTime` (string, required): Start time (ISO 8601)
- `endTime` (string, required): End time (ISO 8601)

**Example:**
```json
{
  "queueIds": ["queue-id-1", "queue-id-2"],
  "startTime": "2026-01-27T00:00:00Z",
  "endTime": "2026-01-27T23:59:59Z"
}
```

### 3. sample_conversations_by_queue

Retrieve a sample of conversation IDs from a queue.

**Parameters:**
- `queueId` (string, required): Queue ID
- `startTime` (string, required): Start time (ISO 8601)
- `endTime` (string, required): End time (ISO 8601)
- `sampleSize` (number, optional): Sample size (default: 10)

**Example:**
```json
{
  "queueId": "queue-id",
  "startTime": "2026-01-27T00:00:00Z",
  "endTime": "2026-01-27T23:59:59Z",
  "sampleSize": 10
}
```

### 4. search_voice_conversations

Search for voice conversations.

**Parameters:**
- `startTime` (string, required): Start time (ISO 8601)
- `endTime` (string, required): End time (ISO 8601)
- `phoneNumber` (string, optional): Phone number filter
- `pageSize` (number, optional): Page size (default: 25)
- `pageNumber` (number, optional): Page number (default: 1)

**Example:**
```json
{
  "startTime": "2026-01-27T00:00:00Z",
  "endTime": "2026-01-27T23:59:59Z",
  "phoneNumber": "+1234567890",
  "pageSize": 25
}
```

### 5. oauth_clients

List all OAuth clients in the organization.

**Parameters:** None

## Authentication

This server uses OAuth 2.0 Client Credentials flow. To set up:

1. Log in to Genesys Cloud Admin
2. Navigate to **Admin → Integrations → OAuth**
3. Create a new OAuth Client
4. Assign required permissions:
   - `routing:queue:view`
   - `analytics:conversationDetail:view`
   - `analytics:conversationAggregate:view`
   - `oauth:client:view`
   - `usage:client:view`
5. Copy Client ID and Secret to your `.env` file

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

### Format

```bash
make fmt
```

## Architecture

```
genesys-cloud-mcp-go/
├── cmd/
│   └── server/          # Main application entry
│       └── main.go
├── pkg/
│   ├── genesys/         # Genesys Cloud API client
│   │   └── client.go
│   └── server/          # MCP server handlers
│       └── handlers.go
├── Dockerfile           # Docker build configuration
├── Makefile            # Build automation
└── README.md           # This file
```

## Limitations

- Transcript retrieval requires async job handling (not implemented)
- Voice call quality, sentiment, and topics tools not yet implemented
- OAuth client usage tool not yet implemented

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.

## Support

- Issues: https://github.com/sabio/genesys-cloud-mcp-go/issues
- Genesys Cloud Docs: https://developer.genesys.cloud/
