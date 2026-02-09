import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Send, Loader2, MoreVertical, History, Plus, Trash2, Moon, Check } from 'lucide-react';
import type {
  ContextEntity,
  CurrentUser,
  DashboardContext,
  EvidenceBundle,
  EvidencePayload,
  HistorySession,
  Message,
  StreamChunk,
  TimelineStep,
  ToolCall,
} from '../types';
import { chatApi, historyApi, userApi } from '../services/api';
import { MarkdownContent } from './MarkdownContent';
import { Artifact, parseArtifacts } from './Artifact';
import { useTheme } from './ThemeProvider';
import { Button } from './ui/button';
import { Avatar, AvatarFallback, AvatarImage } from './ui/avatar';
import { Card, CardContent } from './ui/card';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from './ui/dropdown-menu';
import { Switch } from './ui/switch';
import { cn } from '@/lib/utils';
import { EvidenceModal } from './EvidenceModal';
import { FeedbackModal } from './FeedbackModal';
import { useFeedback } from '../hooks/useFeedback';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from './ui/sheet';
import ContextPicker from './ContextPicker';
import ContextChips from './ContextChips';
import flavioAvatar from '../assets/flavio.png';

interface ChatPanelProps {
  dashboardContext?: DashboardContext;
  onHide?: () => void;
  onNavigate?: (url: string) => void;
}

const SUGGESTIONS = [
  'Summarize what this dashboard is showing',
  'Call out anomalies over the selected time range',
  'Which panels look risky right now?'
];

const STREAMING_STATUS = [
  'Thinking...',
  'Reviewing context...',
  'Using tools...',
  'Crunching the data...',
  'Double-checking results...',
  'Finalizing response...'
];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function truncateText(value: string, maxChars: number): string {
  const text = value.trim();
  if (text.length <= maxChars) {
    return text;
  }
  if (maxChars <= 3) {
    return text.slice(0, maxChars);
  }
  return `${text.slice(0, maxChars - 3)}...`;
}

function summarizeToolArguments(args: Record<string, unknown>): string | undefined {
  const action = typeof args.action === 'string' ? args.action.trim() : '';
  if (action) {
    return `action=${action}`;
  }

  const query = typeof args.query === 'string' ? args.query.trim() : '';
  if (query) {
    return `query=${truncateText(query, 90)}`;
  }

  const expr = typeof args.expr === 'string' ? args.expr.trim() : '';
  if (expr) {
    return `expr=${truncateText(expr, 90)}`;
  }

  const keys = Object.keys(args);
  if (keys.length === 0) {
    return undefined;
  }
  return `${keys.length} argument(s): ${keys.slice(0, 4).join(', ')}`;
}

function toolReasonText(chunk: StreamChunk): string {
  return typeof chunk.reason === 'string' ? truncateText(chunk.reason, 180) : '';
}

function parseToolResultSummary(result: unknown): {
  status: TimelineStep['status'];
  detail: string;
  retryCount: number;
  retryDetail?: string;
} {
  if (!isRecord(result)) {
    if (typeof result === 'string' && result.trim()) {
      return {
        status: 'info',
        detail: truncateText(result, 180),
        retryCount: 0,
      };
    }
    return {
      status: 'info',
      detail: 'Result received.',
      retryCount: 0,
    };
  }

  const statusText = typeof result.status === 'string' ? result.status.trim() : '';
  const messageText = typeof result.message === 'string' ? truncateText(result.message, 180) : '';
  const attemptDetails = Array.isArray(result.attemptDetails) ? result.attemptDetails : [];
  const retryCount = attemptDetails.length > 1 ? attemptDetails.length - 1 : 0;

  let retryDetail = '';
  const syntaxRetry = isRecord(result.syntaxRetry) ? result.syntaxRetry : undefined;
  if (syntaxRetry) {
    const detected = syntaxRetry.detected === true;
    const applied = syntaxRetry.applied === true;
    if (detected && applied) {
      retryDetail = 'Syntax issue detected and one guided correction retry was applied.';
    } else if (detected) {
      retryDetail = 'Syntax issue detected; no safe automatic correction was available.';
    }
  }
  if (!retryDetail && retryCount > 0) {
    retryDetail = `${retryCount} retry attempt(s) were needed before completion.`;
  }

  const parts = [statusText ? `status=${statusText}` : '', messageText].filter(Boolean);
  const detail = parts.length > 0 ? parts.join(' · ') : 'Result received.';

  if (statusText === 'ok') {
    return { status: 'ok', detail, retryCount, retryDetail: retryDetail || undefined };
  }
  if (statusText.includes('error') || statusText === 'tool_unavailable') {
    return { status: 'error', detail, retryCount, retryDetail: retryDetail || undefined };
  }
  if (statusText) {
    return { status: 'warning', detail, retryCount, retryDetail: retryDetail || undefined };
  }
  return { status: 'info', detail, retryCount, retryDetail: retryDetail || undefined };
}

function timelineEventsFromToolChunk(chunk: StreamChunk): Array<Omit<TimelineStep, 'id' | 'order' | 'timestamp'>> {
  const toolName = chunk.tool || 'tool';
  const events: Array<Omit<TimelineStep, 'id' | 'order' | 'timestamp'>> = [];

  if (chunk.arguments && Object.keys(chunk.arguments).length > 0) {
    const reason = toolReasonText(chunk);
    events.push({
      kind: 'tool_call',
      title: `Tool call: ${toolName}`,
      detail: reason || summarizeToolArguments(chunk.arguments),
      tool: toolName,
      status: 'info',
    });
  } else if (toolReasonText(chunk)) {
    events.push({
      kind: 'tool_call',
      title: `Tool call: ${toolName}`,
      detail: toolReasonText(chunk),
      tool: toolName,
      status: 'info',
    });
  }

  if (typeof chunk.result !== 'undefined') {
    const summary = parseToolResultSummary(chunk.result);
    if (summary.retryCount > 0 || summary.retryDetail) {
      events.push({
        kind: 'retry',
        title: `Retry: ${toolName}`,
        detail: summary.retryDetail || `${summary.retryCount} retry attempt(s) detected.`,
        tool: toolName,
        status: 'warning',
      });
    }
    events.push({
      kind: 'tool_result',
      title: `Tool result: ${toolName}`,
      detail: summary.detail,
      tool: toolName,
      status: summary.status,
    });
  }

  return events;
}

function deepCloneUnknown(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => deepCloneUnknown(item));
  }
  if (value && typeof value === 'object') {
    const out: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
      out[key] = deepCloneUnknown(item);
    }
    return out;
  }
  return value;
}

function deepCloneRecord(record: Record<string, unknown>): Record<string, unknown> {
  return deepCloneUnknown(record) as Record<string, unknown>;
}

function cloneEvidencePayload(evidence: EvidencePayload | undefined): EvidencePayload | undefined {
  if (!evidence) {
    return undefined;
  }
  return {
    kb_search: evidence.kb_search
      ? {
          query: evidence.kb_search.query,
          results: evidence.kb_search.results.map((result) => ({ ...result })),
        }
      : undefined,
    vector_search: evidence.vector_search
      ? {
          query: evidence.vector_search.query,
          results: evidence.vector_search.results.map((result) => ({ ...result })),
        }
      : undefined,
  };
}

function buildEvidenceBundle(
  message: Message,
  options: { sessionId?: string; dashboardContext?: DashboardContext }
): EvidenceBundle {
  return {
    schema_version: 'evidence_bundle.v1',
    exported_at: new Date().toISOString(),
    source: 'monitoring-assistant',
    session_id: options.sessionId,
    dashboard_context: options.dashboardContext
      ? (deepCloneUnknown(options.dashboardContext) as DashboardContext)
      : undefined,
    assistant_message: {
      id: message.id,
      role: 'assistant',
      timestamp: message.timestamp,
      content: message.content,
    },
    tool_calls: (message.toolCalls || []).map((call) => ({
      ...call,
      arguments: deepCloneRecord(call.arguments),
      output: deepCloneUnknown(call.output),
    })),
    evidence: cloneEvidencePayload(message.evidence),
    timeline: [...(message.timeline || [])].sort((a, b) => a.order - b.order).map((step) => ({ ...step })),
  };
}

async function copyTextToClipboard(value: string): Promise<boolean> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return true;
  }

  if (typeof document === 'undefined') {
    return false;
  }

  const textarea = document.createElement('textarea');
  textarea.value = value;
  textarea.setAttribute('readonly', 'true');
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  const ok = document.execCommand('copy');
  document.body.removeChild(textarea);
  return ok;
}

function downloadTextFile(filename: string, content: string): void {
  const blob = new Blob([content], { type: 'application/json;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

function evidenceBundleFileName(message: Message): string {
  const safeID = message.id.replace(/[^a-zA-Z0-9_-]/g, '_');
  return `evidence-bundle-${safeID}.json`;
}

export function ChatPanel({ dashboardContext, onHide, onNavigate }: ChatPanelProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [sessionId, setSessionId] = useState<string | undefined>(undefined);
  const [showHistory, setShowHistory] = useState(false);
  const [evidenceModal, setEvidenceModal] = useState<{
    messageId: string;
    type: 'tool' | 'kb' | 'vector' | 'timeline';
  } | null>(null);
  const [feedbackModal, setFeedbackModal] = useState<{ messageId: string } | null>(null);
  const [streamingStatusIndex, setStreamingStatusIndex] = useState(0);
  const [historyItems, setHistoryItems] = useState<HistorySession[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [bundleStatusByMessage, setBundleStatusByMessage] = useState<Record<string, 'copied' | 'exported' | 'error'>>(
    {}
  );
  const [selectedContext, setSelectedContext] = useState<ContextEntity[]>([]);
  const [contextPickerOpen, setContextPickerOpen] = useState(false);
  const [contextPickerAnchor, setContextPickerAnchor] = useState<{ top: number; left: number } | null>(null);
  const [currentUser, setCurrentUser] = useState<CurrentUser | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [authError, setAuthError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const lastNavigateRef = useRef<string>('');
  const inputRef = useRef<HTMLTextAreaElement | null>(null);
  const { theme, setTheme } = useTheme();

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

  const loadCurrentUser = useCallback(async () => {
    setAuthLoading(true);
    setAuthError(null);
    try {
      const user = await userApi.get();
      setCurrentUser(user);
    } catch (error) {
      setCurrentUser(null);
      setAuthError(error instanceof Error ? error.message : 'Failed to confirm Grafana login');
    } finally {
      setAuthLoading(false);
    }
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  useEffect(() => {
    const isStreaming = messages.some((msg) => msg.role === 'assistant' && msg.isStreaming);
    if (!isStreaming) {
      setStreamingStatusIndex(0);
      return;
    }
    const timer = window.setInterval(() => {
      setStreamingStatusIndex((prev) => (prev + 1) % STREAMING_STATUS.length);
    }, 2200);
    return () => {
      window.clearInterval(timer);
    };
  }, [messages]);

  useEffect(() => {
    void loadCurrentUser();
  }, [loadCurrentUser]);

  useEffect(() => {
    if (currentUser || authLoading) {
      return;
    }
    const interval = window.setInterval(() => {
      void loadCurrentUser();
    }, 5000);
    return () => {
      window.clearInterval(interval);
    };
  }, [authLoading, currentUser, loadCurrentUser]);

  const appendMessage = useCallback((message: Message) => {
    setMessages((prev) => [...prev, message]);
  }, []);

  const markBundleStatus = useCallback((messageID: string, status: 'copied' | 'exported' | 'error') => {
    setBundleStatusByMessage((prev) => ({ ...prev, [messageID]: status }));
    window.setTimeout(() => {
      setBundleStatusByMessage((prev) => {
        if (prev[messageID] !== status) {
          return prev;
        }
        const next = { ...prev };
        delete next[messageID];
        return next;
      });
    }, 2000);
  }, []);

  const handleCopyEvidenceBundle = useCallback(
    async (message: Message) => {
      try {
        const bundle = buildEvidenceBundle(message, { sessionId, dashboardContext });
        const serialized = JSON.stringify(bundle, null, 2);
        const copied = await copyTextToClipboard(serialized);
        markBundleStatus(message.id, copied ? 'copied' : 'error');
      } catch (error) {
        console.error('Failed to copy evidence bundle', error);
        markBundleStatus(message.id, 'error');
      }
    },
    [dashboardContext, markBundleStatus, sessionId]
  );

  const handleExportEvidenceBundle = useCallback(
    (message: Message) => {
      try {
        const bundle = buildEvidenceBundle(message, { sessionId, dashboardContext });
        const serialized = JSON.stringify(bundle, null, 2);
        downloadTextFile(evidenceBundleFileName(message), serialized);
        markBundleStatus(message.id, 'exported');
      } catch (error) {
        console.error('Failed to export evidence bundle', error);
        markBundleStatus(message.id, 'error');
      }
    },
    [dashboardContext, markBundleStatus, sessionId]
  );

  const {
    getDraft: getFeedbackDraft,
    updateDraft: updateFeedbackDraft,
    submitFeedback,
    error: feedbackError,
    setError: setFeedbackError,
    submitting: feedbackSubmitting,
  } = useFeedback(sessionId);

  const mergeEvidence = useCallback((current: EvidencePayload | undefined, incoming: EvidencePayload | undefined) => {
    if (!incoming) {
      return current;
    }
    return {
      kb_search: incoming.kb_search ?? current?.kb_search,
      vector_search: incoming.vector_search ?? current?.vector_search,
    };
  }, []);

  const startNewChat = useCallback(() => {
    setMessages([]);
    setSessionId(undefined);
    setShowHistory(false);
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
      if (!messageText.trim() || isLoading || !currentUser) {
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
      const contextForRequest = selectedContext.length > 0 ? [...selectedContext] : undefined;
      setSelectedContext([]);
      setIsLoading(true);

      const assistantId = `${Date.now()}-assistant`;
      const assistantMessage: Message = {
        id: assistantId,
        role: 'assistant',
        content: '',
        timestamp: new Date().toISOString(),
        isStreaming: true,
        toolCalls: [],
        timeline: [],
      };
      appendMessage(assistantMessage);

      try {
        let accumulated = '';
        let toolCalls: ToolCall[] = [];
        let toolCounter = 0;
        let timeline: TimelineStep[] = [];
        let timelineCounter = 0;
        const appendTimelineStep = (step: Omit<TimelineStep, 'id' | 'order' | 'timestamp'>) => {
          timelineCounter += 1;
          const nextStep: TimelineStep = {
            id: `${assistantId}-timeline-${timelineCounter}`,
            order: timelineCounter,
            timestamp: new Date().toISOString(),
            ...step,
          };
          timeline = [...timeline, nextStep];
          setMessages((prev) =>
            prev.map((msg) => (msg.id === assistantId ? { ...msg, timeline: [...timeline] } : msg))
          );
        };
        appendTimelineStep({
          kind: 'start',
          title: 'Assistant started',
          detail: 'Preparing response and checking available context.',
          status: 'info',
        });

        for await (const chunk of chatApi.stream({
          message: messageText,
          session_id: sessionId,
          dashboard_context: dashboardContext,
          selected_context: contextForRequest,
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
            const existingIndex = toolCalls.findIndex((call) => call.id === toolId);
            const existingCall = existingIndex >= 0 ? toolCalls[existingIndex] : undefined;
            const nextArguments =
              chunk.arguments && Object.keys(chunk.arguments).length > 0
                ? chunk.arguments
                : existingCall?.arguments || {};
            const nextReason =
              typeof chunk.reason === 'string' && chunk.reason.trim().length > 0
                ? chunk.reason.trim()
                : existingCall?.reason;
            const toolCall: ToolCall = {
              id: toolId,
              tool: chunk.tool || existingCall?.tool || 'tool',
              reason: nextReason,
              arguments: nextArguments,
              output: chunk.result ?? existingCall?.output,
            };
            if (existingIndex >= 0) {
              toolCalls = toolCalls.map((call) => (call.id === toolId ? toolCall : call));
            } else {
              toolCalls = [...toolCalls, toolCall];
            }
            setMessages((prev) =>
              prev.map((msg) => (msg.id === assistantId ? { ...msg, toolCalls: [...toolCalls] } : msg))
            );

            if ((chunk.tool === 'scratchpad__upsert_panel' || chunk.tool === 'explore__open') && chunk.result && onNavigate) {
              const result = chunk.result as { url?: string };
              if (result.url && result.url !== lastNavigateRef.current) {
                lastNavigateRef.current = result.url;
                onNavigate(result.url);
              }
            }

            const timelineEvents = timelineEventsFromToolChunk(chunk);
            for (const event of timelineEvents) {
              appendTimelineStep(event);
            }
          }

          if (chunk.type === 'evidence') {
            setMessages((prev) =>
              prev.map((msg) =>
                msg.id === assistantId ? { ...msg, evidence: mergeEvidence(msg.evidence, chunk.evidence) } : msg
              )
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
            appendTimelineStep({
              kind: 'error',
              title: 'Response error',
              detail: chunk.message || 'Something went wrong while generating the response.',
              status: 'error',
            });
          }
        }

        if (accumulated.trim().length > 0 && !accumulated.startsWith('Error:')) {
          appendTimelineStep({
            kind: 'final_answer',
            title: 'Final answer',
            detail: truncateText(accumulated, 220),
            status: 'ok',
          });
        }

        setMessages((prev) =>
          prev.map((msg) => (msg.id === assistantId ? { ...msg, isStreaming: false } : msg))
        );
      } catch (error) {
        console.error('Error sending message', error);
        const errorMessage =
          error instanceof Error ? error.message : 'Something went wrong. Check that the server is running.';
        setMessages((prev) =>
          prev.map((msg) =>
            msg.id === assistantId
              ? {
                  ...msg,
                  content: `Error: ${errorMessage}`,
                  isStreaming: false,
                  timeline: [
                    ...(msg.timeline || []),
                    {
                      id: `${assistantId}-timeline-catch-error`,
                      order: (msg.timeline?.length || 0) + 1,
                      kind: 'error',
                      title: 'Response error',
                      detail: errorMessage,
                      status: 'error',
                      timestamp: new Date().toISOString(),
                    },
                  ],
                }
              : msg
          )
        );
      } finally {
        setIsLoading(false);
      }
    },
    [appendMessage, dashboardContext, isLoading, sessionId, currentUser]
  );

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    void sendMessage(input);
  };

  const handleSuggestion = (suggestion: string) => {
    if (!currentUser || authLoading) {
      return;
    }
    setInput(suggestion);
    requestAnimationFrame(() => {
      autoResizeInput();
    });
  };

  const handleContextSelect = useCallback((entity: ContextEntity) => {
    setSelectedContext((prev) => {
      if (prev.some((e) => e.type === entity.type && e.id === entity.id)) return prev;
      return [...prev, entity];
    });
    // Remove the trailing "@" from input
    setInput((prev) => prev.replace(/@\s*$/, '').trimEnd());
    setContextPickerOpen(false);
  }, []);

  const handleContextRemove = useCallback((entity: ContextEntity) => {
    setSelectedContext((prev) => prev.filter((e) => !(e.type === entity.type && e.id === entity.id)));
  }, []);

  const autoResizeInput = useCallback(() => {
    const el = inputRef.current;
    if (!el) {
      return;
    }
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 140)}px`;
  }, []);

  const chatDisabled = authLoading || !currentUser;
  const isDarkMode = theme === 'dark';

  return (
    <div className="chat-panel">
      <div className="w-full max-w-[360px] px-4 py-3 border-b border-border flex items-center justify-between gap-3">
        <div>
          <div className="text-base font-semibold truncate">Powered by Sabio Monitoring</div>
          <div className="text-xs text-muted-foreground mt-0.5">
            {contextLabel}
            {currentUser ? ` · ${currentUser.login}` : ''}
          </div>
        </div>
        <div className="inline-flex items-center gap-2">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={onHide}
            disabled={!onHide}
          >
            Hide chat
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="ghost" size="icon" aria-label="Chat menu">
                <MoreVertical size={16} />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <DropdownMenuLabel>Settings</DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={startNewChat}>
                <Plus size={14} className="mr-2" /> New chat
              </DropdownMenuItem>
              <DropdownMenuItem onClick={openHistory} disabled={chatDisabled}>
                <History size={14} className="mr-2" /> Previous chats
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                className="flex items-center justify-between"
                onSelect={(event) => event.preventDefault()}
              >
                <span className="flex items-center gap-2">
                  <Moon size={14} />
                  Dark mode
                </span>
                <Switch
                  checked={isDarkMode}
                  onCheckedChange={(checked) => setTheme(checked ? 'dark' : 'light')}
                  onClick={(event) => event.stopPropagation()}
                />
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <div className="flex-1 w-full max-w-[360px] overflow-y-auto overflow-x-hidden px-4 py-3 flex flex-col gap-4">
        {messages.length === 0 ? (
          <div className="flex-1 grid place-items-center">
            <Card className="w-full max-w-[320px] text-center shadow-lg">
              {chatDisabled ? (
                <>
                  <CardContent className="p-6 space-y-4">
                    <div className="space-y-2">
                      <h3 className="text-base font-semibold">Waiting for Grafana login</h3>
                      <p className="text-sm text-muted-foreground">
                    {authLoading
                      ? 'Confirming your Grafana session...'
                      : 'Log into Grafana in the left pane to start a chat.'}
                      </p>
                    </div>
                    {authError && <p className="text-red-400 text-sm">{authError}</p>}
                    <div className="flex flex-wrap gap-2 justify-center">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="rounded-full"
                        onClick={() => void loadCurrentUser()}
                        disabled={authLoading}
                      >
                        Retry login
                      </Button>
                    </div>
                  </CardContent>
                </>
              ) : (
                <>
                  <CardContent className="p-6 space-y-4">
                    <div className="grid gap-2 justify-items-center">
                      <Avatar className="assistant-avatar h-14 w-14">
                        <AvatarImage src={flavioAvatar} alt="Flavio" />
                        <AvatarFallback>SF</AvatarFallback>
                      </Avatar>
                      <h3 className="text-base font-semibold">Hi, I'm Flavio</h3>
                      <p className="text-sm text-muted-foreground">
                        Part of the Sabio Monitoring team. Ask me about this dashboard or anything else monitoring related.
                      </p>
                    </div>
                    <div className="flex flex-wrap gap-2 justify-center">
                      {SUGGESTIONS.map((suggestion) => (
                        <Button
                          key={suggestion}
                          type="button"
                          variant="outline"
                          size="sm"
                          className="rounded-full"
                          onClick={() => handleSuggestion(suggestion)}
                        >
                          {suggestion}
                        </Button>
                      ))}
                    </div>
                  </CardContent>
                </>
              )}
            </Card>
          </div>
        ) : (
          messages.map((message) => {
            const isAssistant = message.role === 'assistant';
            const showArtifacts = isAssistant && !message.isStreaming;
            const hasToolCalls = isAssistant && !!message.toolCalls && message.toolCalls.length > 0;
            const hasTimeline = isAssistant && !!message.timeline && message.timeline.length > 0;
            const hasKBEvidence = isAssistant && !!message.evidence?.kb_search?.results?.length;
            const hasVectorEvidence = isAssistant && !!message.evidence?.vector_search?.results?.length;
            const feedbackDraft = getFeedbackDraft(message.id);
            const feedbackSubmitted = feedbackDraft.submitted;
            const bundleStatus = bundleStatusByMessage[message.id];
            const { artifacts, remainingContent } = showArtifacts
              ? parseArtifacts(message.content)
              : { artifacts: [], remainingContent: message.content };

            return (
              <div key={message.id} className={`flex flex-col gap-2 ${message.role}`}>
                <div className="flex flex-col gap-2 w-full">
                  {isAssistant && (
                    <div className="flex justify-start -mb-3 pl-2 z-[1]">
                      <Avatar className="assistant-avatar">
                        <AvatarImage src={flavioAvatar} alt="Flavio" />
                        <AvatarFallback>SF</AvatarFallback>
                      </Avatar>
                    </div>
                  )}
                  <Card
                    className={
                      isAssistant
                        ? 'max-w-full p-3 pt-5'
                        : 'max-w-full p-3 bg-primary/10 border-primary/20'
                    }
                  >
                    {remainingContent.trim().length > 0 && (
                      <MarkdownContent content={remainingContent} />
                    )}
                    {message.isStreaming && (
                      <div className="inline-flex items-center gap-2 text-xs text-muted-foreground mt-2">
                        <Loader2 size={16} className="animate-spin" />
                        {STREAMING_STATUS[streamingStatusIndex]}
                      </div>
                    )}
                  </Card>
                </div>
                {(hasToolCalls || hasTimeline || hasKBEvidence || hasVectorEvidence || (isAssistant && !message.isStreaming)) && (
                  <div className="flex flex-wrap gap-2 pl-2">
                    {hasToolCalls && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="h-7 rounded-full px-3 text-xs"
                        onClick={() => setEvidenceModal({ messageId: message.id, type: 'tool' })}
                      >
                        Tool calls ({message.toolCalls?.length ?? 0})
                      </Button>
                    )}
                    {hasTimeline && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="h-7 rounded-full px-3 text-xs"
                        onClick={() => setEvidenceModal({ messageId: message.id, type: 'timeline' })}
                      >
                        Timeline ({message.timeline?.length ?? 0})
                      </Button>
                    )}
                    {hasKBEvidence && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="h-7 rounded-full px-3 text-xs"
                        onClick={() => setEvidenceModal({ messageId: message.id, type: 'kb' })}
                      >
                        KB search
                      </Button>
                    )}
                    {hasVectorEvidence && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="h-7 rounded-full px-3 text-xs"
                        onClick={() => setEvidenceModal({ messageId: message.id, type: 'vector' })}
                      >
                        Vector search
                      </Button>
                    )}
                    {isAssistant && !message.isStreaming && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="h-7 rounded-full px-3 text-xs"
                        onClick={() => {
                          void handleCopyEvidenceBundle(message);
                        }}
                      >
                        {bundleStatus === 'copied' ? 'Copied bundle' : bundleStatus === 'error' ? 'Copy failed' : 'Copy bundle'}
                      </Button>
                    )}
                    {isAssistant && !message.isStreaming && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="h-7 rounded-full px-3 text-xs"
                        onClick={() => handleExportEvidenceBundle(message)}
                      >
                        {bundleStatus === 'exported' ? 'Exported bundle' : 'Export bundle'}
                      </Button>
                    )}
                    {isAssistant && !message.isStreaming && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="h-7 rounded-full px-3 text-xs"
                        disabled={feedbackSubmitted}
                        onClick={() => {
                          setFeedbackError(null);
                          setFeedbackModal({ messageId: message.id });
                        }}
                      >
                        Feedback
                        {feedbackSubmitted && <Check size={14} className="text-emerald-500" />}
                      </Button>
                    )}
                  </div>
                )}
                {showArtifacts && artifacts.length > 0 && (
                  <div className="grid gap-3">
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

      {selectedContext.length > 0 && (
        <div className="w-full max-w-[360px] px-4 pt-2">
          <ContextChips entities={selectedContext} onRemove={handleContextRemove} />
        </div>
      )}

      <div className="relative w-full max-w-[360px]">
        <ContextPicker
          open={contextPickerOpen}
          anchorRect={contextPickerAnchor}
          onSelect={handleContextSelect}
          onClose={() => setContextPickerOpen(false)}
        />
      </div>

      <form className="w-full max-w-[360px] px-4 py-3 border-t border-border flex gap-2" onSubmit={handleSubmit}>
        <textarea
          ref={inputRef}
          value={input}
          onChange={(event) => {
            const newValue = event.target.value;
            setInput(newValue);
            autoResizeInput();

            // Detect "@" trigger for context picker
            const cursorPos = event.target.selectionStart ?? newValue.length;
            const charBefore = newValue[cursorPos - 1];
            if (charBefore === '@' && (cursorPos === 1 || /\s/.test(newValue[cursorPos - 2] ?? ''))) {
              const rect = inputRef.current?.getBoundingClientRect();
              if (rect) {
                setContextPickerAnchor({ top: rect.top, left: rect.left });
              }
              setContextPickerOpen(true);
            }
          }}
          placeholder="Ask about this dashboard"
          rows={1}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && !event.shiftKey) {
              event.preventDefault();
              void sendMessage(input);
            }
          }}
          disabled={chatDisabled}
          className={cn(
            'flex-1 min-h-[40px] max-h-[140px] resize-none rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm transition-colors placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
            isLoading && 'opacity-80'
          )}
        />
        <Button
          type="submit"
          size="icon"
          disabled={isLoading || !input.trim() || chatDisabled}
          className="h-10 w-10"
        >
          {isLoading ? <Loader2 size={16} /> : <Send size={16} />}
        </Button>
      </form>

      <Sheet open={showHistory} onOpenChange={setShowHistory}>
        <SheetContent side="right">
          <SheetHeader>
            <SheetTitle>Previous chats</SheetTitle>
            <SheetDescription>Pick a conversation or delete it.</SheetDescription>
          </SheetHeader>
          <div className="px-4 pb-6">
            {historyLoading ? (
              <div className="text-muted-foreground text-center py-6 text-sm">Loading...</div>
            ) : historyError ? (
              <div className="text-red-400 text-center py-6 text-sm">{historyError}</div>
            ) : historyItems.length === 0 ? (
              <div className="text-muted-foreground text-center py-6 text-sm">No previous chats yet.</div>
            ) : (
              <div className="grid gap-3">
                {historyItems.map((session) => {
                  const title = session.title || 'Untitled chat';
                  const when = new Date(session.updated_at).toLocaleString();
                  return (
                    <Card key={session.id} className="flex items-center justify-between gap-3 p-3">
                      <button
                        type="button"
                        onClick={() => handleSelectHistory(session)}
                        className="text-left flex-1"
                      >
                        <div className="text-sm font-semibold">{title}</div>
                        <div className="text-xs text-muted-foreground mt-1">{when}</div>
                      </button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        className="text-destructive hover:text-destructive hover:bg-destructive/10"
                        onClick={() => handleDeleteHistory(session)}
                        aria-label="Delete chat"
                      >
                        <Trash2 size={14} />
                      </Button>
                    </Card>
                  );
                })}
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>
      {evidenceModal && (
        <EvidenceModal
          message={messages.find((msg) => msg.id === evidenceModal.messageId)}
          type={evidenceModal.type}
          onClose={() => setEvidenceModal(null)}
        />
      )}
      {feedbackModal && (
        <FeedbackModal
          messageId={feedbackModal.messageId}
          rating={getFeedbackDraft(feedbackModal.messageId).rating}
          comment={getFeedbackDraft(feedbackModal.messageId).comment}
          submitted={getFeedbackDraft(feedbackModal.messageId).submitted}
          submitting={feedbackSubmitting}
          error={feedbackError}
          onClose={() => setFeedbackModal(null)}
          onRate={(value) => updateFeedbackDraft(feedbackModal.messageId, { rating: value })}
          onComment={(value) => updateFeedbackDraft(feedbackModal.messageId, { comment: value })}
          onSubmit={async () => {
            const ok = await submitFeedback(feedbackModal.messageId);
            if (ok) {
              setFeedbackModal(null);
            }
          }}
        />
      )}
    </div>
  );
}
