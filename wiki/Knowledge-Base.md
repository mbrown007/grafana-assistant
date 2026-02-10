# Knowledge Base

The Monitoring Assistant can leverage a knowledge base of markdown files to provide more contextually relevant answers. The KB feature uses a hybrid search model, combining fast keyword-based search with more powerful semantic search.

## Architecture

The KB is divided into two pools, each with its own search method:

| Pool | Search Method | Default Path | Index File | Use Case |
|---|---|---|---|---|
| **Structured** | Token/Keyword | `KB/runbooks` | `.kb_index.json` | Runbooks, metric definitions, alert documentation |
| **Vector** | Embedding Similarity | `KB/platform` | `.kb_vectors.db` | General platform knowledge, scraped documentation |

At query time, both pools are searched (if configured) and the results are merged and ranked to find the most relevant information.

## Data Sources

### Structured KB (Token Search)

This method is ideal for content where keyword matching is crucial.

*   **Path:** `KB/runbooks` (configurable via `kb_structured_path` in `config.yaml`)
*   **Format:** Markdown files (`.md`). Use `##` or `###` headings to define sections.
*   **Indexing:** Run `make kb-reindex-token` to create the `.kb_index.json` file. This does not require an API key.

### Vector KB (Semantic Search)

This method finds relevant content based on meaning, not just keywords. It requires an OpenAI API key to generate embeddings.

You can populate the vector store in two ways:

1.  **Markdown Files:** Place `.md` files in a directory (e.g., `KB/platform`). The content will be split into sections based on headings.
2.  **JSONL from Docs Scraper:** Use the `docs-scraper/` tool to crawl documentation sites and generate a JSONL file. Each line in the file represents a pre-chunked section of content.

To index the vector store, run `make kb-reindex` (for markdown) or `make kb-reindex-jsonl` (for JSONL). This will generate the `.kb_vectors.db` file.

## Integration Methods

You can integrate the KB with the assistant in two ways:

1.  **Direct Context Injection:** The assistant automatically searches the KB and injects the most relevant sections into the LLM prompt with every chat request. This is ideal for providing always-on background context.
2.  **KB MCP Server:** The KB is exposed as a set of tools that the LLM can call on demand. This is useful when KB lookups are only needed occasionally.

You can use both methods simultaneously.

## Configuration

The KB is configured in your `config.yaml` file:

```yaml
# Hybrid KB paths
kb_structured_path: "KB/runbooks"       # Token index source
kb_vector_path: ""                       # Markdown vector source (disabled if empty)
kb_vector_db_path: "KB/.kb_vectors.db"  # SQLite vector store
kb_embedding_model: "text-embedding-3-small"
kb_vector_max_results: 2                # Max vector results per query
```

## Writing Good KB Content

*   **For Token Search:** Use specific, searchable terms in headings and include relevant metric names, PromQL patterns, and alert rule names in the content.
*   **For Vector Search:** Write in natural language, explaining concepts and relationships. Avoid raw config or logs; instead, summarize what they mean.
