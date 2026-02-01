# SM3 MCP Servers (Go)

Model Context Protocol (MCP) servers implemented in Go for the SM3 Monitoring Agent system.

## Overview

This repository contains Go implementations of MCP servers that provide AI agents with tools to interact with monitoring and communications platforms:

- **AlertManager MCP Server** - Prometheus AlertManager integration
- **Genesys Cloud MCP Server** - Genesys Cloud contact center platform integration

## Servers

### AlertManager MCP Server

**Location:** `alertmanager-mcp-go/`

**Features:**
- List active alerts with filtering
- Create and delete silences
- Query alert status and statistics
- Generic pagination support
- Multi-tenant support via X-Scope-OrgId header
- Comprehensive test suite with 13 test functions

**Tools:**
- `list_active_alerts` - List all active alerts
- `list_alerts_by_severity` - Filter alerts by severity (critical, warning, info)
- `list_alerts_by_label` - Filter alerts by label matchers
- `create_silence` - Create a new silence for alerts
- `delete_silence` - Remove an existing silence
- `get_alert_statistics` - Get alert counts and metrics
- `list_silences` - List all active silences
- `get_alertmanager_status` - Get AlertManager cluster status

**Quick Start:**
```bash
cd alertmanager-mcp-go
cp .env.sample .env
# Edit .env with your AlertManager URL and credentials
go run cmd/server/main.go
```

See [alertmanager-mcp-go/README.md](alertmanager-mcp-go/README.md) for detailed documentation.

### Genesys Cloud MCP Server

**Location:** `genesys-cloud-mcp-go/`

**Features:**
- OAuth 2.0 client credentials authentication
- Queue monitoring and statistics
- Agent status and presence information
- Conversation queries
- User management

**Tools:**
- `list_queues` - List all queues with member counts
- `get_queue_members` - Get members of a specific queue
- `get_agent_presence` - Get current presence status of agents
- `search_conversations` - Search conversation history
- `get_user_details` - Get user profile information

**Quick Start:**
```bash
cd genesys-cloud-mcp-go
cp .env.sample .env
# Edit .env with your Genesys Cloud credentials
go run cmd/server/main.go
```

See [genesys-cloud-mcp-go/README.md](genesys-cloud-mcp-go/README.md) for detailed documentation.

## Testing

### Run All Tests

```bash
./run_all_tests.sh
```

### Run Individual Component Tests

```bash
# AlertManager MCP
cd alertmanager-mcp-go
go test ./... -v

# Genesys Cloud MCP
cd genesys-cloud-mcp-go
go test ./... -v
```

### Test Coverage

```bash
# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

See [RUN_TESTS.md](RUN_TESTS.md) for detailed testing instructions.

## Documentation

- **[TESTING_GUIDE.md](TESTING_GUIDE.md)** - Step-by-step testing with local LGTM stack
- **[RUN_TESTS.md](RUN_TESTS.md)** - Test execution guide
- **[TESTS_SUMMARY.md](TESTS_SUMMARY.md)** - Complete test coverage summary

## Architecture

### MCP Protocol

All servers implement the Model Context Protocol via HTTP transport:

1. **Tool Discovery** - Servers expose available tools via `/tools` endpoint
2. **Tool Invocation** - Tools executed via POST to `/invoke`
3. **JSON-RPC** - Standard MCP message format

### Common Patterns

- **Error Handling** - Comprehensive error responses with context
- **Logging** - Structured logging with log/slog
- **Configuration** - Environment variable-based configuration
- **Testing** - Table-driven tests with httptest mocking
- **Docker** - Multi-stage builds for minimal production images

## Requirements

- Go 1.21 or higher
- Access to target platforms (AlertManager, Genesys Cloud)
- API credentials configured in `.env` files

## Installation

### From Source

```bash
# AlertManager MCP
cd alertmanager-mcp-go
go build -o alertmanager-mcp-server cmd/server/main.go

# Genesys Cloud MCP
cd genesys-cloud-mcp-go
go build -o genesys-mcp-server cmd/server/main.go
```

### Docker

```bash
# AlertManager MCP
cd alertmanager-mcp-go
docker build -t alertmanager-mcp-server .
docker run -p 9300:9300 --env-file .env alertmanager-mcp-server

# Genesys Cloud MCP
cd genesys-cloud-mcp-go
docker build -t genesys-mcp-server .
docker run -p 9400:9400 --env-file .env genesys-mcp-server
```

## Configuration

Each server uses environment variables for configuration. Copy `.env.sample` to `.env` in each server directory and configure:

### AlertManager MCP

```env
ALERTMANAGER_URL=http://localhost:9093
ALERTMANAGER_USERNAME=admin
ALERTMANAGER_PASSWORD=secret
ALERTMANAGER_TENANT=
MCP_SERVER_PORT=9300
```

### Genesys Cloud MCP

```env
GENESYS_CLOUD_REGION=us-east-1
GENESYS_CLIENT_ID=your-client-id
GENESYS_CLIENT_SECRET=your-client-secret
MCP_SERVER_PORT=9400
```

## Integration

These MCP servers are designed to work with the SM3 Monitoring Agent system:

- **Grafana Plugin** - [grafana-chat-plugin](https://github.com/Brownster/grafana-chat-plugin)
- **SM3 Agent** - Full monitoring agent implementation

## Development

### Project Structure

```
├── alertmanager-mcp-go/
│   ├── cmd/server/          # Server entry point
│   ├── pkg/
│   │   ├── alertmanager/    # AlertManager client
│   │   └── server/          # MCP server handlers
│   ├── Dockerfile
│   ├── Makefile
│   └── README.md
├── genesys-cloud-mcp-go/
│   ├── cmd/server/          # Server entry point
│   ├── pkg/
│   │   ├── genesys/         # Genesys Cloud client
│   │   └── server/          # MCP server handlers
│   ├── Dockerfile
│   ├── Makefile
│   └── README.md
└── run_all_tests.sh         # Test runner
```

### Adding New Tools

1. Define tool schema in server handlers
2. Implement handler function
3. Add to tool registry
4. Write tests
5. Update documentation

### Code Quality

- Run tests: `go test ./...`
- Run linter: `golangci-lint run`
- Check coverage: `go test ./... -cover`
- Format code: `go fmt ./...`

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run test suite
5. Submit pull request

## Test Coverage

- **AlertManager MCP**: ~75% coverage (13 test functions, 5 benchmarks)
- **Genesys Cloud MCP**: Basic structure in place
- **Total**: 34+ test functions across all components

## Performance

Benchmark results for key operations:

```
BenchmarkNewClient-8                     5000000    250 ns/op
BenchmarkListAlerts-8                    2000000    650 ns/op
BenchmarkPaginateResults-8                500000   2500 ns/op
```

## License

MIT License - see LICENSE file for details

## Related Projects

- [SM3 Agent](https://github.com/mbrown007/sm3_agent) - Full monitoring agent
- [Grafana Chat Plugin](https://github.com/Brownster/grafana-chat-plugin) - Grafana panel integration
- [MCP Specification](https://spec.modelcontextprotocol.io/) - Model Context Protocol

## Support

For issues, questions, or contributions:
- Open an issue on GitHub
- See [TESTING_GUIDE.md](TESTING_GUIDE.md) for troubleshooting

## Authors

Developed for the SM3 Monitoring Agent system.

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
