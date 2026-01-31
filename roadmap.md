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

**Reuse from reference projects:**
| What | Source | Target |
|------|--------|--------|
| OpenAI client with streaming | `grafana-chat-plugin/pkg/llm/openai.go` | `internal/llm/openai.go` |
| MCP client with retry logic | `grafana-chat-plugin/pkg/mcp/client.go` | `internal/mcp/client.go` |
| Tool result formatting | `grafana-chat-plugin/pkg/mcp/formatter.go` | `internal/mcp/formatter.go` |
| Config struct patterns | `grafana-chat-plugin/pkg/plugin/settings.go` | `internal/config/config.go` |

### Phase 2 — Grafana Proxy (de-risked early)
- Reverse proxy `/grafana/*` to Grafana using `net/http/httputil.ReverseProxy`.
- Strip `X-Frame-Options` and `Content-Security-Policy` from proxied responses via `ModifyResponse`.
- Pass through all cookies (Grafana session, auth) transparently.
- Grafana API client to fetch dashboard JSON by UID.
- Context minifier: extract panel titles, descriptions, queries (PromQL/SQL); enforce size limits.
- Cache dashboard context with TTL to avoid repeated API calls.
- **Exit criterion**: Grafana loads correctly in an iframe served by the wrapper, user is authenticated, dashboard JSON can be fetched.

**Reuse:**
| What | Source | Target |
|------|--------|--------|
| Reverse proxy + header stripping | `app_roadmap_discussion.md` `makeGrafanaProxy()` | `internal/proxy/grafana.go` |
| Dashboard context extraction | `app_roadmap_discussion.md` `extractDashboardContext()` | `internal/context/parser.go` |
| Dashboard JSON enrichment | `app_roadmap_discussion.md` `enrichContextFromDashboard()` | `internal/context/enricher.go` |

### Phase 3 — Wrapper UI (Completed)
- React + TypeScript app, built to static assets and embedded in Go binary via `embed`.
- Layout: full-height flex container — Grafana iframe + sliding chat sidebar (0px to 400px push).
- Context parser reads iframe URL for dashboard UID, time range, template variables.
- Frontend message list with SSE streaming response handling.
- Artifact renderer: port components directly from `grafana-chat-plugin/src/components/`:
  - `Artifact.tsx` — chart (bar/line/pie/area via Recharts), table, metric cards, reports.
  - `MarkdownContent.tsx` — markdown rendering with syntax highlighting.
- Tool call display with expandable details (arguments + result).
- **Exit criterion**: Wrapper UI loads Grafana via proxy, chat sidebar opens/closes, context extraction works, artifact components render sample data.

**Reuse:**
| What | Source | Target |
|------|--------|--------|
| Artifact renderer (charts, tables, metrics, reports) | `grafana-chat-plugin/src/components/Artifact.tsx` | `frontend/src/components/Artifact.tsx` |
| Markdown content renderer | `grafana-chat-plugin/src/components/MarkdownContent.tsx` | `frontend/src/components/MarkdownContent.tsx` |
| TypeScript interfaces | `grafana-chat-plugin/src/types.ts` | `frontend/src/types.ts` |
| Chat panel patterns (streaming, tool calls) | `grafana-chat-plugin/src/components/ChatPanel.tsx` | `frontend/src/components/ChatPanel.tsx` |
| API client patterns | `grafana-chat-plugin/src/services/api.ts` | `frontend/src/services/api.ts` |
| Wrapper HTML/CSS layout (iframe + sidebar) | `app_roadmap_discussion.md` `serveWrapper()` | Design reference for React layout |

**Completed artifacts:**
- `frontend/` — Vite + React + TypeScript wrapper UI (chat sidebar, iframe layout, SSE streaming client)
- `frontend/src/components/Artifact.tsx` — artifact renderer (charts, tables, metric cards, reports)
- `frontend/src/components/MarkdownContent.tsx` — markdown renderer
- `frontend/src/components/ChatPanel.tsx` — chat UI + tool call display + demo artifact
- `frontend/src/services/api.ts` — SSE streaming client for `POST /api/chat`
- `frontend/src/utils/dashboard.ts` — URL context parsing (uid, time range, variables)
- `cmd/assistant/main.go` — embeds and serves `static/` assets with SPA fallback

### Phase 4 — Assistant Core
- Chat API `POST /api/chat` with SSE streaming.
- LLM provider interface with OpenAI implementation (port `openai.go` from chat plugin).
- Agent orchestration: conversation loop with tool calling (port pattern from `grafana-chat-plugin/pkg/agent/manager.go`).
- Conversation memory: session-based history with configurable window (port from `agent/memory.go`).
- Prompt assembly: system prompt + dashboard context + user history + tool results.
- MCP client integration: discover and invoke tools from configured MCP servers.
- Tool call visibility in UI (show tool name, arguments, result in collapsible block).
- **Exit criterion**: Streaming chat with OpenAI works, dashboard context is included in prompts, MCP tool calls execute and return, basic artifacts render from LLM responses.

**Reuse:**
| What | Source | Target |
|------|--------|--------|
| Agent orchestration loop | `grafana-chat-plugin/pkg/agent/manager.go` | `internal/agent/manager.go` |
| Conversation memory | `grafana-chat-plugin/pkg/agent/memory.go` | `internal/agent/memory.go` |
| System prompts | `grafana-chat-plugin/pkg/agent/prompts.go` | `internal/agent/prompts.go` |
| SSE streaming handler | `grafana-chat-plugin/pkg/plugin/streaming.go` | `internal/api/streaming.go` |
| MCP tool execution + routing | `grafana-chat-plugin/pkg/plugin/streaming.go` `executeTool()` | `internal/agent/tools.go` |
| Prompt assembly patterns | `sm3_agent` prompt construction | Reference for contextual prompt building |
| MCP server configs | `sm3_agent/mcp_servers.json` pattern | `config.yaml` MCP section |

### Phase 5 — User + History
- Chat history persistence in SQLite keyed by Grafana user ID + org + optional dashboard UID.
- History API: `GET /api/history`, `GET /api/history/:id`, `DELETE /api/history/:id`.
- UI: conversation list in sidebar, load previous conversations, start new conversation.
- Audit log of tool usage and LLM responses (who asked what, what tools ran, cost).

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
- `Makefile` with `build`, `test`, `lint`, `dev` targets.

### Phase 9 — Pilot + Iteration (ongoing)
- Pilot with real dashboards and real users.
- Tune context size limits, prompt templates, artifact rendering.
- Add MCP servers as needed (AlertManager, Genesys Cloud — reuse from `grafana-chat-plugin/mcp_servers/`).
- Gather feedback and iterate.

---

## Proposed Project Structure

```
monitoring-assistant/
├── cmd/
│   └── assistant/
│       └── main.go                 # Entry point
├── internal/
│   ├── config/
│   │   └── config.go              # Config loading (env + file)
│   ├── proxy/
│   │   └── grafana.go             # Reverse proxy for Grafana
│   ├── context/
│   │   ├── parser.go              # URL -> dashboard UID, time range, vars
│   │   └── enricher.go            # Dashboard JSON -> minified context
│   ├── api/
│   │   ├── router.go              # HTTP routing
│   │   ├── chat.go                # POST /api/chat handler
│   │   ├── streaming.go           # SSE streaming
│   │   └── history.go             # Chat history endpoints
│   ├── agent/
│   │   ├── manager.go             # LLM + tool orchestration loop
│   │   ├── memory.go              # Conversation memory
│   │   └── prompts.go             # System prompts
│   ├── llm/
│   │   └── openai.go              # OpenAI client with streaming
│   ├── mcp/
│   │   ├── client.go              # MCP HTTP client
│   │   └── formatter.go           # Tool result formatting
│   ├── storage/
│   │   ├── storage.go             # Storage interface
│   │   └── sqlite.go              # SQLite implementation
│   └── auth/
│       └── session.go             # Grafana session resolution
├── frontend/                       # React + TypeScript
│   ├── src/
│   │   ├── components/
│   │   │   ├── App.tsx
│   │   │   ├── ChatPanel.tsx
│   │   │   ├── Artifact.tsx       # Ported from grafana-chat-plugin
│   │   │   └── MarkdownContent.tsx
│   │   ├── services/
│   │   │   └── api.ts
│   │   └── types.ts
│   ├── package.json
│   └── vite.config.ts
├── static/                         # Built frontend assets (embedded)
├── config.example.yaml
├── Makefile
├── go.mod
└── roadmap.md
```

---

## Risks / Watch Items
- **Cross-origin access**: solved by proxying Grafana under same host via Go reverse proxy.
- **Grafana session reuse**: must pass cookies correctly through proxy; test with both basic auth and OAuth.
- **Context size/cost**: hard caps on dashboard context + summarization to control LLM token usage.
- **Grafana version compatibility**: test with Grafana 10.x and 11.x; iframe behavior may vary.
- **WebSocket proxying**: Grafana uses WebSockets for live updates; reverse proxy must handle `Upgrade` headers.

## Reuse Summary from Reference Apps

### From `grafana-chat-plugin` (primary source — Go + React)
- **Go backend**: OpenAI client, MCP client, agent orchestration, streaming, memory, prompts, tool execution (~1500 lines of proven Go code).
- **React frontend**: Artifact renderer (charts/tables/metrics/reports), markdown renderer, chat panel, SSE handling, TypeScript types (~1000 lines).
- **MCP servers**: AlertManager and Genesys Cloud MCP servers can be deployed alongside.

### From `sm3_agent` (reference patterns — Python)
- **Prompt engineering**: system prompt structure, context injection patterns.
- **MCP orchestration**: multi-MCP routing, tool caching, result formatting patterns.
- **UI patterns**: chat page layout, artifact display, conversation management.
- **Config patterns**: `mcp_servers.json` and `grafana_servers.json` multi-server config structure.

---

## Milestone Exit Criteria
- **Phase 0**: Architecture finalized; Grafana config changes documented (`docs/GRAFANA_CONFIG.md`); data retention defined (30 days); storage choice confirmed (SQLite); artifact schema formalized (`docs/ARTIFACT_SCHEMA.md`); proxy spike validates (reverse proxy works, strips headers, passes cookies — covered by `internal/proxy/grafana_test.go`).
- **Phase 1**: Go binary starts, loads config, connects to SQLite, health endpoint responds, Grafana user can be resolved. ✓ All 22 tests pass.
- **Phase 2**: Grafana loads in iframe via reverse proxy, user is authenticated, dashboard JSON fetched and minified.
- **Phase 3**: Wrapper UI renders, chat sidebar opens/closes, context extracted from URL, artifacts render with sample data.
- **Phase 4**: Streaming chat with OpenAI works end-to-end, dashboard context in prompts, MCP tools execute, artifacts render from LLM.
- **Phase 6**: Security checklist passed; rate limits + CSRF + sanitized output in place.
- **Phase 8**: Single binary with embedded assets runs standalone; systemd unit works; admin docs complete.
