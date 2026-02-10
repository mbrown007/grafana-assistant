# Eval Mock Fixtures

Fixtures in this directory are replayed by `bin/mock-mcp` for deterministic eval runs.

Each fixture is one JSON file:

```json
{
  "tool_name": "grafana__query_prometheus",
  "match": {
    "datasourceUid": "^prometheus$",
    "expr": "node_cpu"
  },
  "response": {
    "status": "success",
    "data": {
      "resultType": "matrix",
      "result": []
    }
  }
}
```

Notes:
- `tool_name` may be omitted when the parent directory name is the tool name.
- `match` values are regex patterns matched against tool argument values.
- If no fixture matches a call, the mock server returns a generic successful "no data" response.
- Use `-server-type <type>` with `bin/mock-mcp` to filter prefixed fixtures (for example, only `grafana__*`).
