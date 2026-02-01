# Chat Flow Sequence Diagram

This diagram illustrates the end-to-end flow of a user's chat message through the Monitoring Assistant, detailing interactions between the frontend, backend, agent, LLM, MCP servers, Knowledge Base (KB), and the SQLite database.

```mermaid
sequenceDiagram
    participant User/Browser
    participant Go Backend (API)
    participant Agent
    participant KB (local)
    participant LLM (OpenAI)
    participant MCP Client
    participant MCP Server
    participant SQLite DB

    User/Browser->>+Go Backend (API): 1. POST /api/chat with user message

    Go Backend (API)->>+Agent: 2. HandleChat(request)
    Note over Agent: Enriches dashboard context

    Agent->>+KB (local): 3. buildKBContext(userMsg, dashCtx)
    KB (local)-->>-Agent: 4. Returns relevant KB sections

    Note over Agent: 5. Builds system prompt with history, user message, dashboard & KB context

    Agent->>+LLM (OpenAI): 6. CreateChatCompletion(prompt, available_tools)

    alt LLM needs more information
        LLM (OpenAI)-->>-Agent: 7. Response (request to call a tool)
        Agent->>+MCP Client: 8. InvokeTool("mcp_server__tool_name", args)
        MCP Client->>+MCP Server: 9. JSON-RPC call
        MCP Server-->>-MCP Client: 10. Tool result
        MCP Client-->>-Agent: 11. Returns tool result

        Note over Agent: 12. Appends tool result to conversation history
        Agent->>+LLM (OpenAI): 13. CreateChatCompletion(prompt, tool_result)
        LLM (OpenAI)-->>-Agent: 14. Final text response
    else LLM answers directly
        LLM (OpenAI)-->>-Agent: 7a. Final text response
    end

    Agent-->>-Go Backend (API): 15. Streams response chunks back
    Go Backend (API)-->>-User/Browser: 16. Streams SSE events to UI

    Agent->>+SQLite DB: 17. Persist full conversation history
    SQLite DB-->>-Agent:

    Go Backend (API)-->>-User/Browser: 18. "done" event
    deactivate Go Backend (API)
```
