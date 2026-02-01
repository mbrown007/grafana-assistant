# KB MCP Server

An MCP server that exposes a local markdown knowledge base (KB) for search and retrieval.

## Features
- Simple keyword search across KB markdown sections
- Fetch full section content by ID
- stdio and SSE transports

## Environment variables

- `KB_PATH` (default: `KB`) — path to KB folder containing markdown files
- `KB_INDEX_PATH` (optional) — path to prebuilt index file (from `kb-reindex`)
- `MCP_TRANSPORT` (default: `stdio`) — `stdio` or `sse`
- `MCP_HOST` (default: `0.0.0.0`) — host for SSE
- `MCP_PORT` (default: `8003`) — port for SSE

## Run (stdio)

```bash
KB_PATH=./KB ./kb-mcp-server
```

If you want faster startup, build the index first:

```bash
make kb-reindex
```

## Run (SSE)

```bash
MCP_TRANSPORT=sse MCP_HOST=127.0.0.1 MCP_PORT=8003 KB_PATH=./KB ./kb-mcp-server
```

## Tools

### search_kb
Search KB sections by keyword.

Input:
- `query` (string, required)
- `limit` (int, optional, default 5)

### get_kb_section
Fetch a KB section by ID returned from `search_kb`.

Input:
- `id` (string, required)
