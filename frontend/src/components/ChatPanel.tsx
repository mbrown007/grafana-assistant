import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Send, Loader2, Wrench, MoreVertical, History, Plus, Trash2, X } from 'lucide-react';
import type { DashboardContext, HistorySession, Message, ToolCall } from '../types';
import { chatApi, historyApi } from '../services/api';
import { MarkdownContent } from './MarkdownContent';
import { Artifact, parseArtifacts } from './Artifact';

interface ChatPanelProps {
  dashboardContext?: DashboardContext;
  onHide?: () => void;
}

const SUGGESTIONS = [
  'Summarize what this dashboard is showing',
  'Call out anomalies over the selected time range',
  'Which panels look risky right now?'
];

export function ChatPanel({ dashboardContext, onHide }: ChatPanelProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [sessionId, setSessionId] = useState<string | undefined>(undefined);
  const [showMenu, setShowMenu] = useState(false);
  const [showHistory, setShowHistory] = useState(false);
  const [historyItems, setHistoryItems] = useState<HistorySession[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  const contextLabel = useMemo(() => {
    if (!dashboardContext) {
      return 'No dashboard context yet';
    }

    const name = dashboardContext.name || dashboardContext.uid;
    const timeFrom = dashboardContext.time_range?.from;
    const timeTo = dashboardContext.time_range?.to;
    const timeLabel = timeFrom || timeTo ? `${timeFrom || 'now-?'} → ${timeTo || 'now'}` : 'Time range unknown';
    return `${name} · ${timeLabel}`;
  }, [dashboardContext]);

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  const appendMessage = useCallback((message: Message) => {
    setMessages((prev) => [...prev, message]);
  }, []);

  const startNewChat = useCallback(() => {
    setMessages([]);
    setSessionId(undefined);
    setShowHistory(false);
    setShowMenu(false);
  }, []);

  const loadHistory = useCallback(async () => {
    setHistoryLoading(true);
    setHistoryError(null);
    try {
      const items = await historyApi.list();
      setHistoryItems(items);
    } catch (error) {
      setHistoryError(error instanceof Error ? error.message : 'Failed to load history');
    } finally {
      setHistoryLoading(false);
    }
  }, []);

  const openHistory = useCallback(() => {
    setShowHistory(true);
    setShowMenu(false);
  }, []);

  const handleSelectHistory = useCallback(async (session: HistorySession) => {
    try {
      const detail = await historyApi.get(session.id);
      const loadedMessages: Message[] = detail.messages.map((msg) => ({
        id: msg.id,
        role: msg.role,
        content: msg.content,
        timestamp: msg.created_at,
      }));
      setMessages(loadedMessages);
      setSessionId(detail.session.id);
      setShowHistory(false);
    } catch (error) {
      setHistoryError(error instanceof Error ? error.message : 'Failed to load chat');
    }
  }, []);

  const handleDeleteHistory = useCallback(
    async (session: HistorySession) => {
      try {
        await historyApi.remove(session.id);
        setHistoryItems((prev) => prev.filter((item) => item.id !== session.id));
        if (sessionId === session.id) {
          startNewChat();
        }
      } catch (error) {
        setHistoryError(error instanceof Error ? error.message : 'Failed to delete chat');
      }
    },
    [sessionId, startNewChat]
  );

  useEffect(() => {
    if (!showHistory) {
      return;
    }
    void loadHistory();
  }, [showHistory, loadHistory]);

  const sendMessage = useCallback(
    async (messageText: string) => {
      if (!messageText.trim() || isLoading) {
        return;
      }

      const userMessage: Message = {
        id: `${Date.now()}-user`,
        role: 'user',
        content: messageText.trim(),
        timestamp: new Date().toISOString(),
      };
      appendMessage(userMessage);
      setInput('');
      setIsLoading(true);

      const assistantId = `${Date.now()}-assistant`;
      const assistantMessage: Message = {
        id: assistantId,
        role: 'assistant',
        content: '',
        timestamp: new Date().toISOString(),
        isStreaming: true,
        toolCalls: [],
      };
      appendMessage(assistantMessage);

      try {
        let accumulated = '';
        let toolCalls: ToolCall[] = [];
        let toolCounter = 0;

        for await (const chunk of chatApi.stream({
          message: messageText,
          session_id: sessionId,
          dashboard_context: dashboardContext,
        })) {
          if (chunk.type === 'start' && chunk.session_id) {
            setSessionId(chunk.session_id);
          }

          if (chunk.type === 'token' && chunk.message) {
            accumulated += chunk.message;
            setMessages((prev) =>
              prev.map((msg) => (msg.id === assistantId ? { ...msg, content: accumulated } : msg))
            );
          }

          if (chunk.type === 'tool') {
            const toolId = chunk.tool_id ?? `${chunk.tool ?? 'tool'}-${toolCounter++}`;
            const toolCall: ToolCall = {
              id: toolId,
              tool: chunk.tool || 'tool',
              arguments: chunk.arguments || {},
              output: chunk.result,
            };
            const existingIndex = toolCalls.findIndex((call) => call.id === toolId);
            if (existingIndex >= 0) {
              toolCalls = toolCalls.map((call) => (call.id === toolId ? toolCall : call));
            } else {
              toolCalls = [...toolCalls, toolCall];
            }
            setMessages((prev) =>
              prev.map((msg) => (msg.id === assistantId ? { ...msg, toolCalls: [...toolCalls] } : msg))
            );
          }

          if (chunk.type === 'complete' && chunk.message && accumulated.trim().length === 0) {
            accumulated = chunk.message;
            setMessages((prev) =>
              prev.map((msg) => (msg.id === assistantId ? { ...msg, content: accumulated } : msg))
            );
          }

          if (chunk.type === 'error') {
            accumulated = `Error: ${chunk.message || 'Something went wrong.'}`;
            setMessages((prev) =>
              prev.map((msg) => (msg.id === assistantId ? { ...msg, content: accumulated } : msg))
            );
          }
        }

        setMessages((prev) =>
          prev.map((msg) => (msg.id === assistantId ? { ...msg, isStreaming: false } : msg))
        );
      } catch (error) {
        console.error('Error sending message', error);
        setMessages((prev) =>
          prev.map((msg) =>
            msg.id === assistantId
              ? {
                  ...msg,
                  content: `Error: ${error instanceof Error ? error.message : 'Something went wrong. Check that the server is running.'}`,
                  isStreaming: false,
                }
              : msg
          )
        );
      } finally {
        setIsLoading(false);
      }
    },
    [appendMessage, dashboardContext, isLoading, sessionId]
  );

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    void sendMessage(input);
  };

  const handleSuggestion = (suggestion: string) => {
    setInput(suggestion);
  };

  return (
    <div className="chat-panel">
      <div className="chat-header">
        <div>
          <div className="chat-title">Assistant</div>
          <div className="chat-subtitle">{contextLabel}</div>
        </div>
        <div className="chat-header-actions">
          <button
            type="button"
            className="hide-chat-button"
            onClick={onHide}
            disabled={!onHide}
          >
            Hide chat
          </button>
          <button
            type="button"
            className="icon-button"
            onClick={() => setShowMenu((prev) => !prev)}
            aria-label="Chat menu"
          >
            <MoreVertical size={16} />
          </button>
          {showMenu && (
            <div className="chat-menu">
              <button type="button" onClick={startNewChat}>
                <Plus size={14} /> New chat
              </button>
              <button type="button" onClick={openHistory}>
                <History size={14} /> Previous chats
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="chat-messages">
        {messages.length === 0 ? (
          <div className="chat-empty">
            <div className="chat-empty-card">
              <h3>Ask about this dashboard</h3>
              <p>Stream answers, tool calls, and artifacts side-by-side with Grafana.</p>
              <div className="chip-row">
                {SUGGESTIONS.map((suggestion) => (
                  <button
                    key={suggestion}
                    type="button"
                    className="chip"
                    onClick={() => handleSuggestion(suggestion)}
                  >
                    {suggestion}
                  </button>
                ))}
              </div>
            </div>
          </div>
        ) : (
          messages.map((message) => {
            const isAssistant = message.role === 'assistant';
            const showArtifacts = isAssistant && !message.isStreaming;
            const { artifacts, remainingContent } = showArtifacts
              ? parseArtifacts(message.content)
              : { artifacts: [], remainingContent: message.content };

            return (
              <div key={message.id} className={`chat-message ${message.role}`}>
                <div className="message-bubble">
                  {remainingContent.trim().length > 0 && (
                    <MarkdownContent content={remainingContent} />
                  )}
                  {message.isStreaming && (
                    <div className="streaming-indicator">
                      <Loader2 size={16} />
                      Streaming response...
                    </div>
                  )}
                </div>
                {message.toolCalls && message.toolCalls.length > 0 && (
                  <div className="tool-calls">
                    {message.toolCalls.map((call) => (
                      <details key={call.id} className="tool-call">
                        <summary>
                          <Wrench size={14} /> {call.tool}
                        </summary>
                        <div className="tool-body">
                          <div>
                            <div className="tool-label">Arguments</div>
                            <pre>{JSON.stringify(call.arguments, null, 2)}</pre>
                          </div>
                          <div>
                            <div className="tool-label">Result</div>
                            <pre>{JSON.stringify(call.output, null, 2)}</pre>
                          </div>
                        </div>
                      </details>
                    ))}
                  </div>
                )}
                {showArtifacts && artifacts.length > 0 && (
                  <div className="artifact-stack">
                    {artifacts.map((artifact, index) => (
                      <Artifact key={`${artifact.type}-${index}`} artifact={artifact} />
                    ))}
                  </div>
                )}
              </div>
            );
          })
        )}
        <div ref={messagesEndRef} />
      </div>

      <form className="chat-input" onSubmit={handleSubmit}>
        <input
          value={input}
          onChange={(event) => setInput(event.target.value)}
          placeholder="Ask about this dashboard..."
          type="text"
        />
        <button type="submit" disabled={isLoading || !input.trim()}>
          {isLoading ? <Loader2 size={16} /> : <Send size={16} />}
        </button>
      </form>

      <div className={`history-panel ${showHistory ? 'open' : ''}`}>
        <div className="history-header">
          <div>
            <div className="history-title">Previous chats</div>
            <div className="history-subtitle">Pick a conversation or delete it.</div>
          </div>
          <button
            type="button"
            className="icon-button"
            onClick={() => setShowHistory(false)}
            aria-label="Close history"
          >
            <X size={16} />
          </button>
        </div>
        <div className="history-body">
          {historyLoading ? (
            <div className="history-status">Loading...</div>
          ) : historyError ? (
            <div className="history-status error">{historyError}</div>
          ) : historyItems.length === 0 ? (
            <div className="history-status">No previous chats yet.</div>
          ) : (
            <div className="history-list">
              {historyItems.map((session) => {
                const title = session.title || 'Untitled chat';
                const when = new Date(session.updated_at).toLocaleString();
                return (
                  <div key={session.id} className="history-item">
                    <button type="button" onClick={() => handleSelectHistory(session)}>
                      <div className="history-item-title">{title}</div>
                      <div className="history-item-meta">{when}</div>
                    </button>
                    <button
                      type="button"
                      className="icon-button danger"
                      onClick={() => handleDeleteHistory(session)}
                      aria-label="Delete chat"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
