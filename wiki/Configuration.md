# Configuration

The Monitoring Assistant is configured using a YAML file (e.g., `config.yaml`). The application also supports overriding configuration values with environment variables and loading secrets from a `.env` file.

## Configuration File

The primary configuration is done through a YAML file. You can create your own `config.yaml` by copying the provided `config.example.yaml`.

### General

| Setting | Environment Variable | Description |
|---|---|---|
| `listen_addr` | `ASSISTANT_LISTEN_ADDR` | The address for the application to listen on (e.g., `:8080`). |
| `base_path` | `ASSISTANT_BASE_PATH` | The base path for serving the UI and API (e.g., `/assistant`). |
| `db_path` | `ASSISTANT_DB_PATH` | The path to the SQLite database file (default: `data/assistant.db`). |
| `data_retention_days` | `ASSISTANT_DATA_RETENTION_DAYS` | The number of days to retain chat history (default: 30). |
| `scratchpad_ttl_days` | | The number of days to keep scratchpad dashboards (default: 7). |
| `scratchpad_folder` | | The name of the Grafana folder to store scratchpad dashboards in. |
| `audit_log_path` | | The path for the JSONL audit log. If empty, file auditing is disabled. |
| `eval_fixture_record_dir` | `ASSISTANT_EVAL_FIXTURE_RECORD_DIR` | Optional directory to record successful MCP tool calls as replay fixtures for eval mock mode. |
| `eval_bypass_auth` | `ASSISTANT_EVAL_BYPASS_AUTH` | Dev/test only: bypass Grafana session auth for `/api/chat` and use a synthetic user (never enable in production). |

### Grafana

| Setting | Environment Variable | Description |
|---|---|---|
| `grafana_url` | `ASSISTANT_GRAFANA_URL` | The base URL of your Grafana instance. |
| `grafana_token` | `ASSISTANT_GRAFANA_TOKEN` | A Grafana service account token for API access. |

### OpenAI

| Setting | Environment Variable | Description |
|---|---|---|
| `openai_api_key` | `ASSISTANT_OPENAI_API_KEY` | Your OpenAI API key. |
| `openai_model` | `ASSISTANT_OPENAI_MODEL` | The OpenAI model to use (default: `gpt-4o`). |

### Model Profile

Model profiles allow prompt/tool prompt variants by model family.

| Setting | Environment Variable | Description |
|---|---|---|
| `model_profile.name` | `ASSISTANT_MODEL_PROFILE` | Explicit profile override (skips family matching). |
| `model_profile.default` | `ASSISTANT_MODEL_PROFILE_DEFAULT` | Fallback profile when no family override matches (default: `balanced`). |
| `model_profile.family_overrides` | | Map of model-family prefixes to profile names; longest matching prefix wins. |

### Request Budget

Per-request budget guardrails limit prompt/completion tokens, tool loop depth, tool calls, and estimated LLM request cost.

| Setting | Environment Variable | Description |
|---|---|---|
| `request_budget.max_prompt_tokens` | `ASSISTANT_REQUEST_BUDGET_MAX_PROMPT_TOKENS` | Estimated prompt-token budget per request (default: `12000`). |
| `request_budget.max_completion_tokens` | `ASSISTANT_REQUEST_BUDGET_MAX_COMPLETION_TOKENS` | Completion-token budget across the request loop (default: `4000`). |
| `request_budget.max_tool_iterations` | `ASSISTANT_REQUEST_BUDGET_MAX_TOOL_ITERATIONS` | Max non-final tool-loop iterations before degrade-to-final-answer mode (default: `5`). |
| `request_budget.max_tool_calls` | `ASSISTANT_REQUEST_BUDGET_MAX_TOOL_CALLS` | Max tool calls per request before halting additional tool execution (default: `12`). |
| `request_budget.max_estimated_cost_usd` | `ASSISTANT_REQUEST_BUDGET_MAX_ESTIMATED_COST_USD` | Max estimated per-request LLM cost in USD (default: `0.10`). |
| `request_budget.prompt_cost_per_1m_usd` | `ASSISTANT_REQUEST_BUDGET_PROMPT_COST_PER_1M_USD` | Prompt token pricing used for cost estimation (USD per 1M tokens). |
| `request_budget.completion_cost_per_1m_usd` | `ASSISTANT_REQUEST_BUDGET_COMPLETION_COST_PER_1M_USD` | Completion token pricing used for cost estimation (USD per 1M tokens). |

### Knowledge Base

| Setting | Environment Variable | Description |
|---|---|---|
| `kb_path` | | The path to the knowledge base folder (default: `KB`). |
| `kb_max_sections` | | The maximum number of KB sections to inject per request. |
| `kb_max_section_chars` | | The maximum number of characters per KB section. |
| `kb_structured_path` | | The path to the structured KB for token-based search. |
| `kb_vector_path` | | The path to markdown files for vector-based search. |
| `kb_vector_db_path` | | The path to the SQLite vector store. |
| `kb_embedding_model` | | The OpenAI model to use for embeddings. |
| `kb_vector_max_results` | | The maximum number of vector search results per query. |
| `kb_dashboard_map` | | A map of dashboard names/UIDs to KB files. |

### Metrics

| Setting | Environment Variable | Description |
|---|---|---|
| `metrics_enabled` | | Enable or disable Prometheus metrics (default: `true`). |

### Security

| Setting | Environment Variable | Description |
|---|---|---|
| `allowed_origin` | | The CORS origin. If blank, it's auto-derived from `listen_addr`. |
| `max_message_length` | | The maximum number of characters for a chat message. |
| `max_body_size` | | The maximum size of a request body in bytes. |
| `rate_limit_per_minute` | | The number of chat requests allowed per user per minute. |
| `rate_limit_burst` | | The burst allowance for the rate limiter. |
| `investigation_tool_timeout_seconds` | | The timeout budget for composite tool actions. |

### Feature Flags

These flags allow controlled rollout and fast rollback of major capabilities.

| Setting | Environment Variable | Description |
|---|---|---|
| `feature_flags.routing_mode` | `ASSISTANT_FEATURE_ROUTING_MODE` | Enable/disable schema/dashboard/KB routing behavior. |
| `feature_flags.sub_agent_mode` | `ASSISTANT_FEATURE_SUB_AGENT_MODE` | Enable/disable coordinator delegation to isolated specialist sub-agent loops. |
| `feature_flags.composite_tool_mode` | `ASSISTANT_FEATURE_COMPOSITE_TOOL_MODE` | Enable/disable `investigation__manage` composite internal tool exposure and execution. |
| `feature_flags.judge_gate_mode` | `ASSISTANT_FEATURE_JUDGE_GATE_MODE` | Enable/disable eval quality-gate enforcement behavior. |
| `feature_flags.evidence_redaction_mode` | `ASSISTANT_FEATURE_EVIDENCE_REDACTION_MODE` | Enable/disable secret redaction for streamed tool/evidence payloads. |

### MCP Servers

The `mcp_servers` section configures the tool integration with Monitoring Control Plane (MCP) servers. You can configure multiple servers, each with a specific `type` and `transport`.

**Transport: `sse`**

For Server-Sent Events (SSE) transport, you need to provide the `url` of the running MCP server.

```yaml
mcp_servers:
  - type: "alertmanager"
    transport: "sse"
    url: "http://localhost:8000"
```

**Transport: `stdio`**

For `stdio` transport, the assistant will spawn the MCP server as a child process. You need to provide the `command` and `args` to execute the server.

```yaml
mcp_servers:
  - type: "alertmanager"
    transport: "stdio"
    command: "/usr/local/bin/alertmanager-mcp-server"
    args: ["-transport", "stdio"]
    env:
      ALERTMANAGER_URL: "http://localhost:9093"
```

You can also filter the tools exposed by each MCP server using `tool_allowlist` and `tool_denylist`.

## Environment Variables and `.env` file

You can override any of the configuration settings using environment variables. The environment variable name is the uppercase version of the setting name, prefixed with `ASSISTANT_`. For example, `listen_addr` can be overridden with `ASSISTANT_LISTEN_ADDR`.

For convenience during development, you can also place secrets and other configuration in a `.env` file in the root of the project. This file will be loaded automatically at startup.
