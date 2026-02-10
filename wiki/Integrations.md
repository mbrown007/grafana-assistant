# Integrations

The Monitoring Assistant is designed to integrate with various tools and systems to provide a comprehensive monitoring experience.

## Grafana Integration

The assistant embeds Grafana in an iframe, and as such, requires some specific Grafana configuration to function correctly.

### `grafana.ini` Configuration

You'll need to make the following changes to your `grafana.ini` file:

*   **Allow Embedding:**
    ```ini
    [security]
    allow_embedding = true
    ```

*   **Cookie SameSite Policy:**
    If the assistant and Grafana share the same origin (via a reverse proxy), set:
    ```ini
    [security]
    cookie_samesite = lax
    ```
    If they are on different origins, you must use:
    ```ini
    [security]
    cookie_samesite = none
    cookie_secure = true
    ```

### Service Account Token

The assistant requires a Grafana service account token to access the Grafana API.

1.  **Create a Service Account:** In Grafana, go to **Administration > Service Accounts**, create a new service account (e.g., `monitoring-assistant`), and assign it the **Viewer** role.
2.  **Generate a Token:** Create a token for the service account and copy it.
3.  **Configure the Assistant:** Add the token to your `config.yaml` or set it as an environment variable:
    ```yaml
    grafana_token: "glsa_your_token_here"
    ```
    ```bash
    export ASSISTANT_GRAFANA_TOKEN="glsa_your_token_here"
    ```

## Knowledge Base (KB) Integration

The assistant can use a knowledge base of markdown files to provide more contextually relevant answers. The KB supports a hybrid search model, combining token-based and vector-based search.

*   **Structured (token-based) Search:** Ideal for runbooks, metric references, and alert documentation where keyword matching is important.
*   **Vector (semantic) Search:** Finds relevant content based on meaning, even if the exact keywords don't match. This is useful for general platform knowledge.

The KB can be integrated in two ways:
1.  **Direct Context Injection:** The assistant automatically searches the KB and injects relevant sections into the LLM prompt.
2.  **KB MCP Server:** The KB is exposed as tools that the LLM can call on demand.

Refer to the `[Knowledge-Base](Knowledge-Base)` page for more details on how to set up and use the KB.

## Monitoring Control Plane (MCP) Servers

The assistant connects to MCP servers to fetch live data from various monitoring tools. These servers expose a set of "tools" that the assistant's AI agent can invoke.

The following MCP servers are available:

| Server | Default Port | Tools |
|---|---|---|
| Alertmanager | 8000 | Alerts, silences, receivers |
| Grafana | 8001 | Dashboards, datasources, Prometheus queries, Loki logs |
| Genesys Cloud | 8002 | Queue volumes, conversations, OAuth clients |
| KB | 8003 | KB search and section retrieval |

MCP servers can be connected via two transport methods, configured in your `config.yaml`:

*   **`sse` (Server-Sent Events):** The assistant connects to a running MCP server over HTTP.
*   **`stdio`:** The assistant spawns the MCP server as a child process.

You can also filter the tools exposed by each MCP server using `tool_allowlist` and `tool_denylist` in the `config.yaml`.
