# Operator Onboarding (Your Grafana Instance)

This guide is for a new engineer/operator who wants to run Monitoring Assistant against their own Grafana instance, then tune quality with KB content, tests, and evals.

## Outcome

After this guide, you will have:

1. A running assistant binary integrated with Grafana.
2. MCP tools connected (Grafana/Alertmanager/KB, optional others).
3. Knowledge base content wired into prompts.
4. A repeatable test + eval process before pushing changes.

## 1) Prerequisites

- Go `1.25+`
- Node.js `18+` (frontend build for packaged binary)
- OpenAI API key
- Grafana with API access (service account token recommended)

Optional (recommended for local integration testing):

- Docker for `dev-test-docker-compose.yml`

## 2) Build binaries

Build assistant only:

```bash
make build
```

Build assistant + frontend embedded for deployment:

```bash
make package
```

Build assistant + all MCP servers:

```bash
make build-all
```

For single-host deployment bundle (assistant + MCP binaries + run script):

```bash
make deploy-lab
```

## 3) Configure Grafana correctly

Minimum Grafana requirements:

- Embedding enabled.
- Subpath support if served through `/grafana`.
- Valid service account token for API calls.

Use this reference:

- `docs/GRAFANA_CONFIG.md`

Important environment/settings examples:

- `GF_SERVER_ROOT_URL=http://<host>:<port>/grafana/`
- `GF_SERVER_SERVE_FROM_SUB_PATH=true`
- `GF_SECURITY_ALLOW_EMBEDDING=true`

## 4) Configure the assistant

Start from example config:

```bash
cp config.example.yaml config.yaml
```

Set secrets in `.env` (auto-loaded):

```bash
ASSISTANT_OPENAI_API_KEY=sk-...
ASSISTANT_GRAFANA_TOKEN=glsa_...
```

At minimum set in `config.yaml`:

- `listen_addr`
- `grafana_url`
- `db_path`
- `mcp_servers`

Reference:

- `wiki/Configuration.md`
- `docs/DEPLOYMENT.md`

## 5) Choose MCP transport mode

### Option A: SSE MCP servers (good for development)

Start MCP servers separately:

```bash
make mcp-start
```

Use SSE entries in `config.yaml`:

```yaml
mcp_servers:
  - type: "grafana"
    transport: "sse"
    url: "http://localhost:8001"
```

### Option B: stdio MCP servers (recommended for single-host production)

Assistant spawns MCP servers as child processes. Start with:

- `deploy/config.stdio.example.yaml`

Use stdio entry pattern:

```yaml
mcp_servers:
  - type: "grafana"
    transport: "stdio"
    command: "/opt/monitoring-assistant/mcp-grafana"
    args: ["-transport", "stdio"]
```

Security hardening tip:

- Keep Grafana MCP read-focused (`--disable-write`, `--disable-admin`) unless you explicitly need write/admin flows.

## 6) Run the assistant

Local dev stack (Docker + MCP + backend + frontend):

```bash
make dev
```

Standalone backend:

```bash
make run
```

Production/systemd layout and unit templates:

- `docs/DEPLOYMENT.md`
- `deploy/monitoring-assistant.service`

## 7) Validate basic health

Check assistant health:

```bash
curl -fsS http://localhost:8081/healthz
```

Check metrics:

```bash
curl -fsS http://localhost:8081/metrics | head
```

Open UI and verify:

1. Grafana iframe loads.
2. Chat answers stream over SSE.
3. Tool calls appear in logs/stream.

## 8) Add knowledge so prompts improve

The assistant can inject KB context into prompts and/or expose KB tools.

Primary KB references:

- `docs/KB_INTEGRATION.md`
- `wiki/Knowledge-Base.md`

### Structured KB (token search)

Place markdown runbooks in:

- `KB/runbooks/`

Reindex:

```bash
make kb-reindex-token
```

### Vector KB (semantic search)

Use markdown in `KB/platform/` or docs-scraper JSONL ingestion.

Reindex:

```bash
make kb-reindex
# or
make kb-reindex-jsonl
```

### KB settings that directly affect prompt context

- `kb_max_sections`
- `kb_max_section_chars`
- `kb_structured_path`
- `kb_vector_db_path`
- `kb_vector_max_results`
- `kb_dashboard_map`

Tip:

- Use `kb_dashboard_map` to pin dashboard-specific runbooks so the right KB appears in context when users are on that dashboard.

## 9) Prompt/behavior tuning knobs

Use these for controlled tuning without code changes:

### Model profile

- `model_profile.default` (`balanced`, `compact`, `strict`)
- `model_profile.family_overrides`

### Request budget guardrails

- `request_budget.max_prompt_tokens`
- `request_budget.max_completion_tokens`
- `request_budget.max_tool_iterations`
- `request_budget.max_tool_calls`
- `request_budget.max_estimated_cost_usd`

### Feature flags

- `feature_flags.routing_mode`
- `feature_flags.sub_agent_mode`
- `feature_flags.composite_tool_mode`
- `feature_flags.judge_gate_mode`
- `feature_flags.evidence_redaction_mode`

Roll out changes by enabling one flag at a time and watching metrics in `docs/OBSERVABILITY.md`.

## 10) Run tests before push

Backend/unit:

```bash
make test
```

Backend + frontend:

```bash
make test-all
```

Integration and E2E (optional in CI/local):

```bash
make test-integration
make e2e-up
make test-e2e
make e2e-down
```

## 11) Run evals before push

### Fast deterministic local gate (recommended first)

```bash
make eval-baseline-mock
RUN_ARTIFACT="$(ls -1t tests/evals/results/baseline-*.json | head -n1)"
make eval-judge RUN_ARTIFACT="$RUN_ARTIFACT"
make eval-guard RUN_ARTIFACT="$RUN_ARTIFACT"

JUDGE_REPORT="$(ls -1t tests/evals/results/judge-*.json | head -n1)"
GUARD_REPORT="$(ls -1t tests/evals/results/guard-*.json | head -n1)"
make eval-quality-gate \
  JUDGE_REPORT="$JUDGE_REPORT" \
  GUARD_REPORT="$GUARD_REPORT"
```

### Live baseline against running assistant

Requires chat auth (typically Grafana session cookie):

```bash
export ASSISTANT_COOKIE='grafana_session=...'
make eval-baseline
```

See details:

- `docs/evals/README.md`
- `docs/EVAL_CASE_AUTHORING.md`

## 12) Recommended rollout checklist

1. `make test` is green.
2. `make eval-baseline-mock` + judge/guard/gate are green.
3. Feature flags set for intended rollout stage.
4. Grafana dashboard imported from `docs/agent_monitoring.json`.
5. Prompt/tool/sub-agent metrics look healthy.
6. Only then push and deploy.

## 13) Troubleshooting quick hits

- Grafana blank in iframe: verify `allow_embedding` + subpath config (`docs/GRAFANA_CONFIG.md`).
- Tool calls missing: verify MCP server health and `mcp_servers` config transport/URLs.
- KB not appearing: check paths and run `make kb-reindex-token`/`make kb-reindex`.
- Eval baseline auth failures: set `ASSISTANT_COOKIE` or use `make eval-baseline-mock`.
- Cost/latency spikes: tighten `request_budget.*`, then inspect prompt/tool-count metrics by intent.
