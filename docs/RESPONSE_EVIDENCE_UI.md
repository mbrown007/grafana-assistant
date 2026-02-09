# Response Evidence UI (Tool Calls + KB + Vector)

## Goal
Surface per-response evidence used by the agent: tool calls, KB search, and vector search. Show small buttons (“chips”) at the bottom of each assistant bubble only when that evidence exists. Clicking a chip opens a modal/popup with the detailed data for that response.

## Scope (Phase 1)
- UI chips: `Tool Calls`, `KB Search`, `Vector Search`
- Chips appear only when corresponding data exists for that assistant reply.
- Clicking chip opens a modal/popup that displays full details.
- Store evidence **in memory only** (no persistence).

## Non-Goals (Phase 1)
- Persistence across page refresh
- Export / share evidence
- Complex filtering or search inside evidence

## UX / UI Requirements
- Chips are visually subtle and compact (footer of assistant bubble).
- Modal uses scrollable content area for large payloads.
- Modal allows copy of raw JSON payload.
- Modal includes a short title and response timestamp.

## Data Model (Frontend)
Extend the chat message type to include optional evidence payloads:

```ts
export type EvidencePayload = {
  kbSearch?: KbSearchEvidence
  vectorSearch?: VectorSearchEvidence
}

export type AssistantMessage = {
  id: string
  role: 'assistant'
  content: string
  createdAt: string
  toolCalls?: ToolCall[] // Added directly to AssistantMessage
  evidence?: EvidencePayload
}
```

Suggested evidence structures (minimal to start):

```ts
export type ToolCall = {
  id: string
  tool: string
  arguments: Record<string, unknown>
  output?: unknown
}

export type KbSearchEvidence = {
  query: string
  results: Array<{
    path: string
    excerpt: string
    score?: number
  }>
}

export type VectorSearchEvidence = {
  query: string
  results: Array<{
    docId: string
    chunkId: string
    excerpt: string
    score?: number
  }>
}
```

## Data Flow (Backend -> Frontend)
- When the backend returns an assistant message, include `evidence` only if it exists.
- For Phase 1, evidence is stored in memory in the backend and attached to the response.
- Do not persist evidence in DB yet.

## Redaction Policy (Phase 4)
- Streamed tool arguments/results and KB/vector evidence payloads are redacted before sending SSE chunks to the frontend.
- Redaction is key-aware and pattern-aware:
  - Key-aware: fields with names containing terms like `token`, `secret`, `password`, `api_key`, `authorization`, `client_secret`, `refresh_token`, `private_key`, `credential`.
  - Pattern-aware: bearer/basic auth strings, JWT-like tokens, and inline key-value patterns such as `token=...`, `password: ...`, `api_key=...`.
- Mask value: `[REDACTED]`.
- Scope note: redaction is applied to frontend stream payloads; backend audit persistence remains unchanged unless explicitly configured otherwise.
- Implementation reference: `internal/agent/redaction.go`.

## UI Components
1) **ChatBubbleFooter**
   - Reads `message.evidence`
   - Renders chips only for present keys

2) **EvidenceModal**
   - Accepts `type`, `payload`, `messageId`
   - Renders formatted JSON and human-friendly sections

Chip behavior:
- Click opens modal with payload
- Close button + ESC to dismiss

## Implementation Steps (Suggested)
1) **Backend**
   - Identify where tool calls and searches are executed.
   - Collect evidence into a structured object.
   - Attach evidence to assistant message in the response.

2) **Frontend**
   - Extend message type to include `evidence`.
   - Add chips in assistant message footer.
   - Implement modal with scrollable content and JSON view.

3) **Testing**
   - Unit tests for evidence rendering.
   - Manual test with responses that include tool calls and KB/vector searches.

## Open Questions
- Should evidence be redacted (secrets) before sending to frontend?
- Should tool calls be collapsed by default inside modal?
- Do we want separate tabs in modal or a single combined view?
