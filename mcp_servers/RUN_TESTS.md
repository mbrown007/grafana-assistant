# Running Tests

This document explains how to run tests for all SM3 components.

## Quick Start - Run All Tests

```bash
# From sm3_agent root directory
./run_all_tests.sh
```

## Individual Component Tests

### 1. Grafana SM3 Chat Plugin Tests

```bash
cd grafana-sm3-chat-plugin

# Run all tests
go test ./... -v

# Run specific package tests
go test ./pkg/mcp -v
go test ./pkg/agent -v
go test ./pkg/plugin -v

# Run with coverage
go test ./... -cover

# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run benchmarks
go test ./... -bench=. -benchmem
```

### 2. AlertManager MCP Server Tests

```bash
cd mcps/alertmanager-mcp-go

# Run all tests
go test ./... -v

# Run specific tests
go test ./pkg/alertmanager -v
go test ./pkg/server -v

# Run with coverage
go test ./... -cover

# Run benchmarks
go test ./pkg/server -bench=BenchmarkPaginateResults -benchmem
```

### 3. Genesys Cloud MCP Server Tests

```bash
cd mcps/genesys-cloud-mcp-go

# Run all tests
go test ./... -v

# Run specific tests
go test ./pkg/genesys -v
go test ./pkg/server -v

# Run with coverage
go test ./... -cover
```

## Test Coverage Goals

| Component | Current Coverage | Goal |
|-----------|------------------|------|
| Grafana Plugin | ~70% | 80% |
| AlertManager MCP | ~75% | 85% |
| Genesys MCP | ~60% | 75% |

## Integration Tests

### Prerequisites

1. Start LGTM stack:
```bash
cd /home/marc/Documents/docker-otel-lgtm
./run-lgtm.sh
```

2. Start MCP servers (see TESTING_GUIDE.md)

### Run Integration Tests

```bash
cd grafana-sm3-chat-plugin

# Run integration tests (requires running services)
go test ./... -tags=integration -v
```

## Continuous Integration

### GitHub Actions Workflow

Create `.github/workflows/test.yml`:

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'

    - name: Test Grafana Plugin
      run: |
        cd grafana-sm3-chat-plugin
        go test ./... -v -cover

    - name: Test AlertManager MCP
      run: |
        cd mcps/alertmanager-mcp-go
        go test ./... -v -cover

    - name: Test Genesys MCP
      run: |
        cd mcps/genesys-cloud-mcp-go
        go test ./... -v -cover
```

## Test Structure

### Grafana Plugin Tests

```
grafana-sm3-chat-plugin/
├── pkg/
│   ├── mcp/
│   │   ├── client.go
│   │   └── client_test.go          ✅ Created
│   ├── agent/
│   │   ├── memory.go
│   │   ├── memory_test.go          ✅ Created
│   │   ├── manager.go
│   │   └── manager_test.go         (TODO)
│   ├── llm/
│   │   ├── openai.go
│   │   └── openai_test.go          (TODO)
│   └── plugin/
│       ├── resources.go
│       ├── resources_test.go       ✅ Created
│       ├── streaming.go
│       └── streaming_test.go       (TODO)
```

### AlertManager MCP Tests

```
alertmanager-mcp-go/
├── pkg/
│   ├── alertmanager/
│   │   ├── client.go
│   │   └── client_test.go          ✅ Created
│   └── server/
│       ├── handlers.go
│       ├── handlers_test.go        (TODO)
│       ├── pagination.go
│       └── pagination_test.go      ✅ Created
```

### Genesys MCP Tests

```
genesys-cloud-mcp-go/
├── pkg/
│   ├── genesys/
│   │   ├── client.go
│   │   └── client_test.go          (TODO)
│   └── server/
│       ├── handlers.go
│       └── handlers_test.go        (TODO)
```

## Test Patterns Used

### 1. Table-Driven Tests

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"case 1", "input1", "output1", false},
        {"case 2", "input2", "output2", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### 2. Mock HTTP Servers

```go
func TestHTTPClient(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Mock response
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"result": "success"}`))
    }))
    defer server.Close()

    client := NewClient(server.URL)
    // Test client...
}
```

### 3. Concurrent Testing

```go
func TestConcurrency(t *testing.T) {
    var wg sync.WaitGroup
    wg.Add(100)

    for i := 0; i < 100; i++ {
        go func() {
            defer wg.Done()
            // Concurrent operation
        }()
    }

    wg.Wait()
    // Verify results
}
```

### 4. Benchmarks

```go
func BenchmarkFunction(b *testing.B) {
    // Setup
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        Function()
    }
}
```

## Common Test Commands

### Run Specific Test

```bash
go test -run TestName
go test -run TestName/SubtestName
```

### Run Tests with Race Detector

```bash
go test -race ./...
```

### Run Tests with Timeout

```bash
go test -timeout 30s ./...
```

### Generate Test Report

```bash
go test ./... -json > test-report.json
```

### Run Tests in Parallel

```bash
go test ./... -parallel 4
```

## Troubleshooting

### Tests Fail with "connection refused"

- Ensure LGTM stack is running
- Check MCP servers are started
- Verify ports are not blocked

### Tests Timeout

- Increase timeout: `go test -timeout 60s`
- Check for deadlocks with `-race` flag
- Review test dependencies

### Coverage Not Generated

```bash
# Ensure output directory exists
mkdir -p coverage

# Generate coverage
go test ./... -coverprofile=coverage/coverage.out

# View in browser
go tool cover -html=coverage/coverage.out
```

## Next Steps

### Additional Tests to Create

1. **Plugin Integration Tests**:
   - End-to-end chat flow
   - Dashboard context extraction
   - Tool execution

2. **MCP Handler Tests**:
   - Tool handler logic
   - Error handling
   - Response formatting

3. **LLM Client Tests**:
   - OpenAI streaming
   - Tool call handling
   - Error recovery

4. **Performance Tests**:
   - Load testing
   - Memory profiling
   - Concurrency testing

### Test Automation

- Set up pre-commit hooks for tests
- Configure CI/CD pipeline
- Add code coverage reporting
- Implement test result notifications

## Resources

- [Go Testing Documentation](https://pkg.go.dev/testing)
- [Table-Driven Tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [httptest Package](https://pkg.go.dev/net/http/httptest)
- [Test Coverage](https://go.dev/blog/cover)
