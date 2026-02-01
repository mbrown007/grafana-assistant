export interface DashboardTimeRange {
  from: string;
  to: string;
}

export interface DashboardContext {
  uid: string;
  name?: string;
  folder?: string;
  tags?: string[];
  time_range?: DashboardTimeRange;
  variables?: Record<string, string>;
}

export interface ChatRequest {
  message: string;
  session_id?: string;
  dashboard_context?: DashboardContext;
}

export interface ChatResponse {
  response: string;
  session_id: string;
}

export interface StreamChunk {
  type: 'start' | 'token' | 'tool' | 'error' | 'complete' | 'done';
  message?: string;
  session_id?: string;
  tool?: string;
  tool_id?: string;
  arguments?: Record<string, unknown>;
  result?: unknown;
}

export interface ToolCall {
  id: string;
  tool: string;
  arguments: Record<string, unknown>;
  output?: unknown;
}

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
  isStreaming?: boolean;
  toolCalls?: ToolCall[];
}

export interface HistorySession {
  id: string;
  title: string;
  dashboard_uid?: string;
  created_at: string;
  updated_at: string;
}

export interface HistoryMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  created_at: string;
}

export interface HistoryDetail {
  session: HistorySession;
  messages: HistoryMessage[];
}

export interface CurrentUser {
  id: number;
  login: string;
  name: string;
  email?: string;
  org_id: number;
}
