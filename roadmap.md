# Grafana Assistant Wrapper — Roadmap

## Goals
- Build a Go-based wrapper portal around Grafana OSS with an assistant comparable to Grafana Cloud.
- Single binary deployment (no Docker).
- Security-first, customer-facing.
- Chat history tied to Grafana user, visible only inside the wrapper UI.

## Key Decisions (Locked)
- **Identity**: use Grafana login/session; no separate auth system.
- **LLM**: direct OpenAI first; architect to swap to Grafana LLM App later.
- **UI surface**: wrapper UI only (iframe + chat sidebar); no external admin UI.
- **Frontend stack**: React + TypeScript (reuse components from `grafana-chat-plugin`).
- **Artifact schema**: formalized before Phase 2; contract between Go backend and React UI. See `docs/ARTIFACT_SCHEMA.md`.
- **MCP transport**: SSE protocol for all MCP servers (alertmanager, grafana, genesys cloud).

## Assumptions
- Wrapper runs on same host as Grafana/Prometheus/Alertmanager.
- Grafana can be embedded or proxied to allow iframe usage.
- Service account token available for Grafana API access (dashboard JSON).

## INFO
- Grafana test instance is on http://127.0.0.1:3000

---

## Phased Roadmap

### Phase 0 — Alignment & Schema Definition (Completed)
- Finalize architecture and scope boundaries (wrapper vs plugin).
- Confirm Grafana config changes needed (`grafana.ini`: `allow_embedding = true`, CSP, cookie settings).
- Define data retention window for chat history.
- Choose storage (SQLite first; keep interface for Postgres).
- **Formalize the Artifact Schema** — define the JSON contract between Go backend and frontend before any UI work begins. Base it on the proven schema from `grafana-chat-plugin`. See `docs/ARTIFACT_SCHEMA.md` for the canonical definitions.
- **Spike: Grafana proxy proof-of-concept** — validate that `httputil.ReverseProxy` correctly proxies Grafana, passes auth cookies, and strips `X-Frame-Options`/CSP headers. This de-risks the highest technical uncertainty early.

**Completed artifacts:**
- `.gitignore`, `go.mod`, `Makefile`, `config.example.yaml`
- `cmd/assistant/main.go` — entry point with proxy, health, and test page
- `internal/config/config.go` — config loader (YAML + env overrides)
- `internal/proxy/grafana.go` + `grafana_test.go` — reverse proxy with header stripping (tested)
- `internal/api/types.go` — Go types from `docs/ARTIFACT_SCHEMA.md`
- `internal/storage/storage.go` — `Store` interface stub
- `docs/GRAFANA_CONFIG.md` — Grafana configuration requirements

### Phase 1 — Foundation (Completed)
- Go project scaffolding: module init, directory layout, config loader (env + file), structured logging (`slog`), health endpoints.
- Storage interface + initial SQLite implementation (chat history, sessions).
- Grafana API client (service account token auth) — fetch `/api/user`, `/api/dashboards/uid/:uid`.
- Session-aware user resolution via proxied Grafana session cookie.
- Basic unit test harness.
- **Artifact schema contract tests** — round-trip JSON marshal/unmarshal tests for `ArtifactData`, `StreamChunk`, `ChatRequest`, and `ChatResponse` against canonical examples from `docs/ARTIFACT_SCHEMA.md`. Prevents UI/backend drift.

**Completed artifacts:**
- `internal/storage/storage.go` — `Store` interface with sessions, messages, and purge methods
- `internal/storage/sqlite.go` + `sqlite_test.go` — SQLite implementation with WAL mode, auto-migration, cascade deletes (4 tests)
- `internal/grafana/client.go` + `client_test.go` — Grafana API client: `GetCurrentUser`, `ResolveUserFromSession`, `GetDashboard` (4 tests)
- `internal/auth/session.go` + `session_test.go` — Session resolver: extracts cookies from request, resolves Grafana user (2 tests)
- `internal/llm/openai.go` — OpenAI client with streaming (ported from reference, adapted for direct OpenAI API)
- `internal/mcp/client.go` — MCP HTTP client with JSON-RPC, tool discovery, retry logic (ported from reference)
- `internal/mcp/formatter.go` + `formatter_test.go` — Tool result formatting for LLM consumption (5 tests)
- `internal/api/types_test.go` — Artifact schema contract tests: round-trip JSON for all types against canonical examples (8 tests)
- `internal/config/config.go` — Extended with `db_path`, `openai_api_key`, `openai_model`, `mcp_servers` fields
- `cmd/assistant/main.go` — Updated with SQLite init, Grafana client setup, graceful shutdown (signal handling)
- `config.example.yaml` — Updated with all new configuration fields

### Phase 2 — Grafana Proxy (Completed)
- Reverse proxy `/grafana/*` to Grafana using `net/http/httputil.ReverseProxy`.
- Strip `X-Frame-Options` and `Content-Security-Policy` from proxied responses via `ModifyResponse`.
- Pass through all cookies (Grafana session, auth) transparently.
- Grafana API client to fetch dashboard JSON by UID.
- Context minifier: extract panel titles, descriptions, queries (PromQL/SQL); enforce size limits.
- Cache dashboard context with TTL to avoid repeated API calls.

### Phase 3 — Wrapper UI (Completed)
- React + TypeScript app, built to static assets and embedded in Go binary via `embed`.
- Layout: full-height flex container — Grafana iframe + sliding chat sidebar (0px to 400px push).
- Context parser reads iframe URL for dashboard UID, time range, template variables.
- Frontend message list with SSE streaming response handling.
- Artifact renderer: chart (bar/line/pie/area via Recharts), table, metric cards, reports.
- Markdown renderer with inline formatting.
- Tool call display with expandable details (arguments + result).

**Completed artifacts:**
- `frontend/` — Vite + React + TypeScript wrapper UI
- `frontend/src/components/Artifact.tsx` — artifact renderer (charts, tables, metric cards, reports)
- `frontend/src/components/MarkdownContent.tsx` — markdown renderer
- `frontend/src/components/ChatPanel.tsx` — chat UI with SSE streaming, tool call display
- `frontend/src/services/api.ts` — SSE streaming client for `POST /api/chat`
- `frontend/src/utils/dashboard.ts` — URL context parsing (uid, time range, variables)
- `cmd/assistant/main.go` — embeds and serves `static/` assets with SPA fallback

### Phase 4 — Assistant Core (Completed)
- Chat API `POST /api/chat` with SSE streaming.
- Agent orchestration: conversation loop with tool calling (max 5 iterations).
- Conversation memory: sliding window (20 messages) with session-based history from SQLite.
- System prompt assembly: role definition, dashboard context (panels, queries, time range, variables), artifact schema with examples, tool descriptions. Dynamically adapts based on whether tools are available.
- MCP client rewritten for SSE transport protocol (persistent GET /sse connection + POST /message with session ID). Supports the `mcp-go` library used by all three MCP servers.
- MCP tool routing: discovers tools at startup, prefixes with server type, routes calls to correct server.
- Config loader reads `.env` file automatically for secrets (OpenAI key, etc.).
- `make dev` workflow: starts MCP servers + Go backend + Vite frontend in one command.

**Completed artifacts:**
- `internal/agent/prompts.go` — system prompt builder with dashboard context, time range, variables, tool descriptions, artifact examples; adapts behavior for tools-available vs no-tools
- `internal/agent/memory.go` — sliding window conversation memory with storage reload
- `internal/agent/tools.go` — MCP-to-OpenAI tool conversion + tool call routing
- `internal/agent/manager.go` — agent orchestration loop (LLM → tool calls → re-prompt → stream)
- `internal/api/streaming.go` + `streaming_test.go` — SSE HTTP handler with explicit 200 flush (4 tests)
- `internal/mcp/client.go` — rewritten for MCP SSE transport (connect, session management, async response dispatch)
- `internal/config/config.go` — `.env` auto-loading (best-effort, won't override real env vars)
- `cmd/assistant/main.go` — wires LLM client, MCP clients (connect + discover), agent manager, `POST /api/chat` route
- `internal/agent/prompts_test.go` — tests for prompt generation (5 tests)
- `internal/agent/memory_test.go` — tests for memory window + history loading (4 tests)
- `internal/agent/tools_test.go` — tests for MCP-to-OpenAI conversion (2 tests)
- `config.yaml` — three MCP servers configured (alertmanager :8000, grafana :8001, genesyscloud :8002)
- `.env` — secrets file with OpenAI key + MCP server env vars
- `Makefile` — `dev`, `dev-stop`, `dev-restart`, `dev-logs`, `mcp-build`, `mcp-start`, `mcp-stop` targets
- `README.md` — full project documentation with all make commands

**MCP servers integrated:**
| Server | Port | Source | Tools |
|--------|------|--------|-------|
| Alertmanager | 8000 | `mcp_servers/alertmanager-mcp-go/` | alerts, silences, receivers (8 tools) |
| Grafana | 8001 | `mcp_servers/mcp-grafana/` | dashboards, datasources, Prometheus queries, Loki logs, alerting, incidents (60+ tools) |
| Genesys Cloud | 8002 | `mcp_servers/genesys-cloud-mcp-go/` | queues, conversations, OAuth clients (5 tools) |

### Phase 5 — User + History
- Chat history persistence in SQLite keyed by Grafana user ID + org + optional dashboard UID.
- History API: `GET /api/history`, `GET /api/history/:id`, `DELETE /api/history/:id`.
- UI: conversation list in sidebar, load previous conversations, start new conversation.
- Audit log of tool usage and LLM responses (who asked what, what tools ran, cost).

**Completed artifacts:**
- `internal/storage/storage.go` — sessions now include `dashboard_uid`; audit log entries added to Store interface
- `internal/storage/sqlite.go` + `sqlite_test.go` — schema migration for `dashboard_uid` + `audit_log`, filtering, audit insert tests
- `internal/api/history.go` + `history_test.go` — history list/detail/delete endpoints with user scoping
- `internal/api/types.go` — history DTOs + SSE `session_id` in stream chunks
- `internal/agent/manager.go` — user-bound session creation, audit logging, streamed session IDs
- `cmd/assistant/main.go` — history endpoints wired + user resolver in chat flow
- `frontend/src/components/ChatPanel.tsx` — three-dot menu, slide-in history list, delete/start new chat
- `frontend/src/services/api.ts` + `frontend/src/types.ts` — history API client + types
- `frontend/src/styles.css` — history panel + menu styling

### Phase 6 — Security Hardening
- CSRF protection (SameSite cookies + CSRF token on state-changing requests).
- Cookie flags: `HttpOnly`, `Secure`, `SameSite=Strict` for wrapper session.
- CORS: restrict to wrapper origin only.
- Input validation: max message length, sanitize user input before prompt injection.
- Output sanitization: sanitize LLM responses before rendering (XSS prevention).
- Prompt injection mitigation: system prompt boundaries, input/output separation.
- Secrets management: file permissions on config, env var support, optional encrypted config.
- Rate limits per user on chat API (token bucket or sliding window).

### Phase 7 — Observability + QA
- Prometheus metrics: request latency, LLM call duration, token usage/cost, tool call counts, error rates.
- Structured logs with correlation IDs (request ID through proxy -> API -> LLM -> MCP).
- Integration tests with Grafana OSS + MCP servers (test proxy, context extraction, chat flow).
- Frontend tests for chat + artifact rendering.

### Phase 8 — Packaging + Docs
- Single binary packaging with `go:embed` for all static assets.
- Example `config.yaml` with documented options.
- systemd unit file for deployment.
- Admin docs: Grafana `grafana.ini` requirements, network/firewall requirements, security posture, data retention config.

### Phase 9 — Pilot + Iteration (ongoing)
- Pilot with real dashboards and real users.
- Tune context size limits, prompt templates, artifact rendering.
- Gather feedback and iterate.

---

## Project Structure

```
monitoring-assistant/
├── cmd/
│   └── assistant/
│       └── main.go                 # Entry point + embedded frontend
├── internal/
│   ├── agent/
│   │   ├── manager.go             # LLM + tool orchestration loop
│   │   ├── memory.go              # Conversation memory (sliding window)
│   │   ├── prompts.go             # System prompt assembly
│   │   └── tools.go               # MCP-to-OpenAI conversion + routing
│   ├── api/
│   │   ├── types.go               # Chat request/response, stream chunks, artifacts
│   │   └── streaming.go           # SSE HTTP handler
│   ├── auth/
│   │   └── session.go             # Grafana session resolution
│   ├── config/
│   │   └── config.go              # YAML + env + .env config loader
│   ├── context/
│   │   ├── parser.go              # URL -> dashboard UID, time range, vars
│   │   └── enricher.go            # Dashboard JSON -> minified context
│   ├── grafana/
│   │   └── client.go              # Grafana API client
│   ├── llm/
│   │   └── openai.go              # OpenAI client with streaming
│   ├── mcp/
│   │   ├── client.go              # MCP client (SSE transport protocol)
│   │   └── formatter.go           # Tool result formatting
│   ├── proxy/
│   │   └── grafana.go             # Reverse proxy for Grafana
│   └── storage/
│       ├── storage.go             # Storage interface
│       └── sqlite.go              # SQLite implementation
├── frontend/                       # React + TypeScript + Vite
│   └── src/
│       ├── components/            # ChatPanel, Artifact, MarkdownContent
│       ├── services/              # SSE streaming API client
│       └── utils/                 # Dashboard URL parsing
├── mcp_servers/
│   ├── alertmanager-mcp-go/       # AlertManager MCP server
│   ├── mcp-grafana/               # Grafana MCP server
│   └── genesys-cloud-mcp-go/      # Genesys Cloud MCP server
├── static/                         # Built frontend assets (embedded)
├── config.yaml                     # Local config
├── config.example.yaml             # Documented config template
├── .env                            # Secrets (gitignored)
├── Makefile                        # Build + dev workflow
└── README.md
```

---

## Risks / Watch Items
- **Cross-origin access**: solved by proxying Grafana under same host via Go reverse proxy.
- **Grafana session reuse**: must pass cookies correctly through proxy; test with both basic auth and OAuth.
- **Context size/cost**: hard caps on dashboard context + summarization to control LLM token usage.
- **Grafana version compatibility**: test with Grafana 10.x and 11.x; iframe behavior may vary.
- **WebSocket proxying**: Grafana uses WebSockets for live updates; reverse proxy must handle `Upgrade` headers.
- **MCP SSE connection stability**: persistent SSE connections may drop; client handles reconnection gracefully by skipping unavailable servers.

---

## Milestone Exit Criteria
- **Phase 0**: Architecture finalized; Grafana config documented; artifact schema formalized; proxy spike validates. **Done.**
- **Phase 1**: Go binary starts, loads config, connects to SQLite, health endpoint responds, Grafana user resolved. All tests pass. **Done.**
- **Phase 2**: Grafana loads in iframe via reverse proxy, authenticated, dashboard JSON fetched and minified. **Done.**
- **Phase 3**: Wrapper UI renders, chat sidebar opens/closes, context extracted from URL, artifacts render with sample data. **Done.**
- **Phase 4**: Streaming chat works end-to-end, dashboard context in prompts, MCP tools execute and return, artifacts render from LLM. `make dev` starts everything. **Done.**
- **Phase 5**: Chat history persisted, history API works, conversation list in UI. **Done.**
- **Phase 6**: Security checklist passed; rate limits + CSRF + sanitized output in place.
- **Phase 7**: Prometheus metrics emitted; integration tests pass.
- **Phase 8**: Single binary with embedded assets runs standalone; systemd unit works; admin docs complete.
