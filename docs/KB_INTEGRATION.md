# Knowledge Base (KB) Integration

The Monitoring Assistant can leverage a knowledge base of markdown files to provide more contextually relevant answers. The KB can be integrated in two ways:

1.  **Direct Context Injection:** The assistant can directly search a local KB folder and inject the most relevant sections into the LLM prompt.
2.  **KB MCP Server:** The KB can be exposed as a set of tools via a dedicated MCP server. The LLM can then choose to call these tools to search the KB.

## KB Data Types and Formats

The assistant supports two KB data types:

1.  **Structured KB articles (token search).**
2.  **Vector KB sections (semantic search).**

Both data types are authored as Markdown and are parsed the same way: files are split into sections at `##` and `###` headings. Each section becomes a searchable unit with a title (the heading text) and body (the content until the next heading).

### 1) Structured KB articles (token search)

**Purpose:** Fast keyword search and direct context injection.  
**Default path:** `KB/runbooks` (config: `kb_structured_path`).  
**Index file:** `KB/runbooks/.kb_index.json` (or `kb_path/.kb_index.json` if using legacy `kb_path`).

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

### 2) Vector KB sections (semantic search)

**Purpose:** Meaning-based retrieval (useful when exact keywords do not match).  
**Path:** `kb_vector_path` (empty = disabled).  
**Vector DB:** `kb_vector_db_path` (default: `KB/.kb_vectors.db`).  
**Embedding model:** `kb_embedding_model` (default: `text-embedding-3-small`).

**Format requirements:**

- Markdown files (`.md`) only.
- Same sectioning rules as structured KB articles (`##` / `###` headings).
- Content should be natural language; avoid large logs or tables in a single section.
- Vector indexing requires an OpenAI API key (`ASSISTANT_OPENAI_API_KEY` or `OPENAI_API_KEY`).

**Indexing:**

Run `make kb-reindex` to build/update:

- Token index: `KB/runbooks/.kb_index.json`
- Vector index: `KB/.kb_vectors.db` (if `kb_vector_path` is set and API key is available)

## Example KB Runbook

See `KB/runbooks/example_service_latency.md` for a complete example.

## Direct Context Injection

This is the simplest way to use the KB. When the `kb_path` is configured in `config.yaml`, the assistant will automatically search the KB on every chat request.

### How it works

1.  **Indexing:** On the first request, the assistant builds an in-memory index of all the markdown files in the `kb_path` directory. The markdown files are split into sections based on headings (`##` and `###`).
2.  **Query Weighting:** For each chat request, the assistant builds a weighted search query from:
    *   The user's message (weight: 1)
    *   The dashboard tags (weight: 3)
    *   The panel titles and queries (weight: 4)
    *   The dashboard variables (weight: 2)
3.  **Search:** The assistant searches the KB index using the weighted query and finds the top N most relevant sections.
4.  **Context Injection:** The content of the top sections is then formatted and prepended to the system prompt that is sent to the LLM.

### Configuration

To enable direct context injection, set the `kb_path` in `config.yaml`:

```yaml
# config.yaml
kb_path: "KB"
```

## KB MCP Server

The KB MCP server exposes the KB as a set of tools that the LLM can call. This gives the LLM more control over when and how to search the KB.

### Tools

The KB MCP server provides the following tools:

*   **`search_kb(query: string, limit: int) -> list[dict]`**: Searches the KB and returns a list of matching sections. Each section is a dictionary containing the `id`, `title`, `path`, `snippet`, and `score`.
*   **`get_kb_section(id: string) -> dict`**: Retrieves the full content of a KB section by its ID.

### How it works

1.  **Tool Registration:** The KB MCP server is registered with the agent like any other MCP server. The agent then makes the `search_kb` and `get_kb_section` tools available to the LLM.
2.  **LLM Tool Call:** The LLM can choose to call these tools if it determines that it needs more information to answer a question. For example, it might call `search_kb` with a query based on the user's message.
3.  **Tool Execution:** The agent executes the tool call and returns the result to the LLM.
4.  **LLM Response:** The LLM then uses the information from the tool call to generate a response to the user.

### Configuration

To enable the KB MCP server, add it to the `mcp_servers` list in `config.yaml`:

```yaml
# config.yaml
mcp_servers:
  - type: "kb"
    transport: "sse"
    url: "http://localhost:8003"
```

## Choosing Between the Two

*   **Direct Context Injection** is simpler to set up and is a good choice if you want the KB to be searched automatically on every request. It's a "fire and forget" way to provide more context to the LLM.
*   The **KB MCP Server** is more flexible and gives the LLM more control. It's a good choice if you want the LLM to be able to search the KB more deliberately and to be able to ask for specific sections of the KB. It can also be more efficient, as the KB is only searched when the LLM decides it's necessary.

You can use both methods at the same time, but it's generally recommended to choose one or the other to avoid confusion. If both are enabled, the direct context injection will happen on every request, and the LLM will also have the option to call the KB tools.
