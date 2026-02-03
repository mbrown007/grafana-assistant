# Knowledge Base (KB) Integration

The Monitoring Assistant can leverage a knowledge base of markdown files to provide more contextually relevant answers. The KB can be integrated in two ways:

1.  **Direct Context Injection:** The assistant automatically searches the KB and injects the most relevant sections into the LLM prompt.
2.  **KB MCP Server:** The KB is exposed as tools via a dedicated MCP server. The LLM can then call these tools to search the KB on demand.

---

## KB Architecture

The KB uses a **hybrid search** model with two disjoint pools:

| Pool | Search method | Path | Index file | Use case |
|---|---|---|---|---|
| **Structured (runbooks)** | Token/keyword matching | `KB/runbooks/` | `KB/runbooks/.kb_index.json` | Metrics references, runbooks, alert docs |
| **Vector (platform)** | Embedding similarity | `KB/platform/` or JSONL | `KB/.kb_vectors.db` | General platform knowledge, scraped docs |

At query time both pools are searched (if configured) and results are merged by normalized score.

```
KB/
  runbooks/                              # Token-based search
    Genesys_Cloud/
      genesys_cloud_metrics_overview.md
    example_service_latency.md
  platform/                              # Vector-based search (markdown)
    Genesys_Cloud/
      genesys_cloud_overview.md
  .kb_index.json                         # Token index (auto-generated)
  .kb_vectors.db                         # Vector index (auto-generated)
```

---

## Data Sources

### 1) Structured KB articles (token search)

**Purpose:** Fast keyword search and direct context injection.
**Default path:** `KB/runbooks` (config: `kb_structured_path`).
**Index file:** `KB/runbooks/.kb_index.json`.

**Format requirements:**

- Markdown files (`.md`) only.
- Use `##` / `###` headings to define sections.
- Keep sections focused (1–3 screens of text); smaller sections rank better.
- Include the most relevant terms in headings and first lines (improves scoring).

**Example folder layout:**

```
KB/
  runbooks/
    redis_latency.md
    api_error_spike.md
```

**Indexing (no API key needed):**

```bash
make kb-reindex-token
# or: go run ./cmd/kb-reindex -structured-path KB/runbooks
```

### 2) Vector KB sections (semantic search)

**Purpose:** Meaning-based retrieval — finds relevant content even when exact keywords don't match.
**Requires:** An OpenAI API key for generating embeddings.

There are two ways to populate the vector store:

#### Option A: Markdown files

Place `.md` files in `KB/platform/` (or any directory). Same sectioning rules as structured articles: content is split at `##` / `###` headings.

```bash
make kb-reindex
# or: go run ./cmd/kb-reindex -structured-path KB/runbooks -vector-path KB/platform
```

#### Option B: JSONL from the docs scraper

The `docs-scraper/` tool crawls documentation sites and produces a JSONL file where each line is a pre-chunked section:

```json
{"id": "abc123", "text": "## Overview\nContent...", "metadata": {"source": "GenesysCloud", "url": "https://...", "title": "Page Title", "section_path": "Page > Heading"}}
```

To ingest JSONL into the vector store:

```bash
make kb-reindex-jsonl
# or: go run ./cmd/kb-reindex -structured-path KB/runbooks -vector-jsonl docs-scraper/genesys_chunks.jsonl
```

Both options can be combined — the vector store merges all sections by ID. Sections whose content hasn't changed (by SHA-256 hash) are skipped, and sections no longer present in the source are deleted.

---

## End-to-End: Scraping Docs into the Vector Store

1. **Set up the scraper:**

   ```bash
   cd docs-scraper
   python -m venv .venv && source .venv/bin/activate
   pip install -r requirements.txt   # httpx, beautifulsoup4, readability-lxml, markdownify, tqdm
   ```

2. **Run the scraper:**

   ```bash
   python scraper.py \
     --base "https://all.docs.genesys.com/GenesysCloud/" \
     --out genesys_chunks.jsonl \
     --max-pages 8000
   ```

   For authenticated pages, pass cookies (see `docs-scraper/README.md`).

3. **Ingest into the vector store:**

   ```bash
   export ASSISTANT_OPENAI_API_KEY="sk-..."
   make kb-reindex-jsonl
   ```

   This reads the JSONL, embeds each chunk via the OpenAI API, and stores them in `KB/.kb_vectors.db`. Incremental: unchanged chunks are skipped.

4. **Verify:**

   ```bash
   # Check section count
   sqlite3 KB/.kb_vectors.db "SELECT COUNT(*) FROM kb_vectors;"
   ```

5. **Start the assistant** — vector search is active automatically when the DB exists and an API key is set.

---

## Configuration

### config.yaml

```yaml
# Legacy KB path (backward compat, used if kb_structured_path is empty)
kb_path: "KB"
kb_max_sections: 2
kb_max_section_chars: 2000

# Hybrid KB paths
kb_structured_path: "KB/runbooks"       # Token index source (default)
kb_vector_path: ""                       # Markdown vector source (empty = disabled)
kb_vector_db_path: "KB/.kb_vectors.db"  # SQLite vector store
kb_embedding_model: "text-embedding-3-small"
kb_vector_max_results: 2                # Max vector results per query
```

### Environment variables

| Variable | Description |
|---|---|
| `ASSISTANT_KB_STRUCTURED_PATH` | Override `kb_structured_path` |
| `ASSISTANT_KB_VECTOR_PATH` | Override `kb_vector_path` |
| `ASSISTANT_KB_VECTOR_DB_PATH` | Override `kb_vector_db_path` |
| `ASSISTANT_KB_EMBEDDING_MODEL` | Override `kb_embedding_model` |
| `ASSISTANT_KB_VECTOR_MAX_RESULTS` | Override `kb_vector_max_results` |
| `ASSISTANT_OPENAI_API_KEY` | Required for vector search (embedding + query) |

### kb-reindex CLI flags

```
-structured-path   Path to structured KB folder (default: KB/runbooks)
-out               Token index output path (default: <structured-path>/.kb_index.json)
-vector-path       Path to markdown files for vector indexing
-vector-jsonl      Path to JSONL file from docs scraper
-vector-db         Vector SQLite DB path (default: KB/.kb_vectors.db)
-embedding-model   OpenAI embedding model (default: text-embedding-3-small)
-openai-api-key    API key (or set ASSISTANT_OPENAI_API_KEY / OPENAI_API_KEY)
-kb-path           Alias for -structured-path (backward compat)
```

---

## Makefile Targets

| Target | Description |
|---|---|
| `make kb-reindex` | Rebuild both token + vector indexes (markdown sources) |
| `make kb-reindex-token` | Rebuild token index only (no API key needed) |
| `make kb-reindex-jsonl` | Rebuild token index + ingest scraper JSONL into vector store |

---

## How Search Works at Runtime

1. **Token search:** The user message, dashboard tags, panel titles, and queries are tokenized and weighted. The structured index is searched by keyword overlap.
2. **Vector search:** The user message is embedded via the OpenAI API and compared by cosine similarity against all stored vectors.
3. **Merge:** Token scores are normalized to 0–1 (divided by max score). Vector scores are already 0–1. Results below 0.3 similarity are filtered. The merged list is sorted by score and truncated.
4. **Injection:** The top results are formatted and appended to the user message as `[KB Context]`.

### Graceful degradation

- **No API key:** Vector search is disabled. Token search works normally. Single info log at startup.
- **No vector DB file:** Vector search is disabled. Single warning log.
- **Embedding API fails at query time:** Warning logged, token-only results returned.
- **Embedding API fails during reindex:** Error logged per chunk, remaining chunks still processed.

---

## KB MCP Server

The KB MCP server exposes the KB as tools that the LLM can call on demand.

### Tools

| Tool | Description |
|---|---|
| `search_kb` | Keyword search over structured runbooks. Returns `{id, title, path, snippet, score}`. |
| `get_kb_section` | Fetch full content of a section by ID. |
| `search_kb_semantic` | Semantic search over vector store. Only registered if vector index + embedder are available. |

### MCP server environment variables

| Variable | Description |
|---|---|
| `KB_PATH` | Path to structured KB folder |
| `KB_INDEX_PATH` | Pre-built token index path (optional) |
| `KB_VECTOR_DB_PATH` | Vector SQLite DB path (enables `search_kb_semantic`) |
| `KB_EMBEDDING_MODEL` | Embedding model (default: `text-embedding-3-small`) |
| `ASSISTANT_OPENAI_API_KEY` | Required for `search_kb_semantic` |

### Configuration

```yaml
mcp_servers:
  - type: "kb"
    transport: "sse"
    url: "http://localhost:8003"
```

---

## Direct Context Injection

### How it works

1.  **Indexing:** On the first request, the assistant builds an in-memory index from the structured KB path. Pre-built indexes (`.kb_index.json`) are loaded if available.
2.  **Query Weighting:** For each chat request, a weighted search query is built from:
    *   The user's message (weight: 1)
    *   Dashboard tags (weight: 3)
    *   Panel titles and queries (weight: 4)
    *   Dashboard variables (weight: 2)
    *   Explore queries (weight: 3)
3.  **Search:** Both token and vector indexes are queried and results are merged.
4.  **Context Injection:** The top sections are formatted and appended to the user message.

---

## Writing Good KB Content

### For runbooks (token search)

- Use specific, searchable terms in headings: `### Edge Collector (edge)` is better than `### Infrastructure`.
- Include metric names, PromQL patterns, and alert rule names in the body — these are the tokens that get matched.
- Keep sections self-contained; each should make sense on its own.

### For platform knowledge (vector search)

- Write in natural language. Vector search matches meaning, not just keywords.
- Explain concepts, relationships, and context — "what is X and why does it matter."
- Avoid dumping raw config or logs; summarize what they mean.
- Aim for 250–900 tokens per chunk (the scraper's defaults). Sections that are too short lack context; sections that are too long dilute relevance.

---

## Choosing Between the Two Integration Methods

*   **Direct Context Injection** is simpler and automatic. Every chat request gets relevant KB context without the LLM needing to decide. Best for always-relevant background knowledge.
*   The **KB MCP Server** gives the LLM control over when to search. Better when KB lookups are only sometimes needed, or when the LLM should be able to drill into specific sections.

You can use both simultaneously. Direct injection happens on every request; MCP tools are available for the LLM to call additionally.
