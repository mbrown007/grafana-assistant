import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Send, Loader2, Sparkles, Wrench } from 'lucide-react';
import type { DashboardContext, Message, ToolCall } from '../types';
import { chatApi } from '../services/api';
import { MarkdownContent } from './MarkdownContent';
import { Artifact, parseArtifacts } from './Artifact';

interface ChatPanelProps {
  dashboardContext?: DashboardContext;
}

const SUGGESTIONS = [
  'Summarize what this dashboard is showing',
  'Call out anomalies over the selected time range',
  'Which panels look risky right now?'
];

const SAMPLE_ARTIFACT = `Here is a quick snapshot based on the current dashboard context.

\`\`\`artifact
{
  "type": "report",
  "title": "Daily Monitoring Summary",
  "subtitle": "2026-01-31",
  "sections": [
    {
      "type": "summary",
      "title": "Executive Summary",
      "content": "Latency is stable overall, but error spikes appear in the EU region during the last hour."
    },
    {
      "type": "metrics",
      "metrics": [
        { "label": "Active Alerts", "value": 4, "icon": "alert", "color": "red" },
        { "label": "p95 Latency", "value": "310ms", "icon": "activity", "color": "amber" },
        { "label": "Healthy Services", "value": "11/12", "icon": "server", "color": "green" }
      ]
    },
    {
      "type": "chart",
      "title": "Error Rate (last 6h)",
      "chartType": "line",
      "data": [
        { "name": "00:00", "errors": 12 },
        { "name": "02:00", "errors": 9 },
        { "name": "04:00", "errors": 22 },
        { "name": "06:00", "errors": 15 }
      ]
    }
  ]
}
\`\`\`
`;

export function ChatPanel({ dashboardContext }: ChatPanelProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [sessionId] = useState(() => `session-${Date.now()}`);
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
                  content: 'The chat service is not available yet. Try again after Phase 4 is implemented.',
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

  const handleDemo = () => {
    const demoMessage: Message = {
      id: `${Date.now()}-demo`,
      role: 'assistant',
      content: SAMPLE_ARTIFACT,
      timestamp: new Date().toISOString(),
      isStreaming: false,
    };
    appendMessage(demoMessage);
  };

  return (
    <div className="chat-panel">
      <div className="chat-header">
        <div>
          <div className="chat-title">Assistant</div>
          <div className="chat-subtitle">{contextLabel}</div>
        </div>
        <button className="ghost-button" onClick={handleDemo} type="button">
          <Sparkles size={16} />
          Load demo
        </button>
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
    </div>
  );
}
