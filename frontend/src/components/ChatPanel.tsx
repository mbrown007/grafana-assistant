import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Send, Loader2, MoreVertical, History, Plus, Trash2, Moon, X, Star, Check } from 'lucide-react';
import type {
  CurrentUser,
  DashboardContext,
  EvidencePayload,
  EvidenceResult,
  HistorySession,
  Message,
  ToolCall,
} from '../types';
import { chatApi, feedbackApi, historyApi, userApi } from '../services/api';
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
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from './ui/sheet';
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

export function ChatPanel({ dashboardContext, onHide, onNavigate }: ChatPanelProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [sessionId, setSessionId] = useState<string | undefined>(undefined);
  const [showHistory, setShowHistory] = useState(false);
  const [evidenceModal, setEvidenceModal] = useState<{
    messageId: string;
    type: 'tool' | 'kb' | 'vector';
  } | null>(null);
  const [feedbackModal, setFeedbackModal] = useState<{ messageId: string } | null>(null);
  const [feedbackDrafts, setFeedbackDrafts] = useState<
    Record<string, { rating: number; comment: string; submitted: boolean }>
  >({});
  const [feedbackError, setFeedbackError] = useState<string | null>(null);
  const [feedbackSubmitting, setFeedbackSubmitting] = useState(false);
  const [streamingStatusIndex, setStreamingStatusIndex] = useState(0);
  const [historyItems, setHistoryItems] = useState<HistorySession[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState<string | null>(null);
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

  const getFeedbackDraft = useCallback(
    (messageId: string) =>
      feedbackDrafts[messageId] ?? { rating: 0, comment: '', submitted: false },
    [feedbackDrafts]
  );

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
            const existingIndex = toolCalls.findIndex((call) => call.id === toolId);
            const existingCall = existingIndex >= 0 ? toolCalls[existingIndex] : undefined;
            const nextArguments =
              chunk.arguments && Object.keys(chunk.arguments).length > 0
                ? chunk.arguments
                : existingCall?.arguments || {};
            const toolCall: ToolCall = {
              id: toolId,
              tool: chunk.tool || existingCall?.tool || 'tool',
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

            if (chunk.tool === 'scratchpad__upsert_panel' && chunk.result && onNavigate) {
              const result = chunk.result as { url?: string };
              if (result.url && result.url !== lastNavigateRef.current) {
                lastNavigateRef.current = result.url;
                onNavigate(result.url);
              }
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
            const hasKBEvidence = isAssistant && !!message.evidence?.kb_search?.results?.length;
            const hasVectorEvidence = isAssistant && !!message.evidence?.vector_search?.results?.length;
            const feedbackDraft = getFeedbackDraft(message.id);
            const feedbackSubmitted = feedbackDraft.submitted;
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
                {(hasToolCalls || hasKBEvidence || hasVectorEvidence || (isAssistant && !message.isStreaming)) && (
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

      <form className="w-full max-w-[360px] px-4 py-3 border-t border-border flex gap-2" onSubmit={handleSubmit}>
        <textarea
          ref={inputRef}
          value={input}
          onChange={(event) => {
            setInput(event.target.value);
            autoResizeInput();
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
      {evidenceModal && (() => {
        const selectedMessage = messages.find((msg) => msg.id === evidenceModal.messageId);
        const title =
          evidenceModal.type === 'tool'
            ? 'Tool calls'
            : evidenceModal.type === 'kb'
              ? 'KB search'
              : 'Vector search';

        return (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
            <div className="w-full max-w-[720px] max-h-[80vh] overflow-hidden rounded-lg border border-border bg-background shadow-xl">
              <div className="flex items-center justify-between border-b px-4 py-3">
                <div className="text-sm font-semibold">{title}</div>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Close evidence"
                  onClick={() => setEvidenceModal(null)}
                >
                  <X size={16} />
                </Button>
              </div>
              <div className="p-4 overflow-y-auto max-h-[72vh] space-y-4">
                {evidenceModal.type === 'tool' && (
                  <>
                    {(selectedMessage?.toolCalls || []).length === 0 ? (
                      <div className="text-sm text-muted-foreground">No tool calls recorded for this response.</div>
                    ) : (
                      selectedMessage?.toolCalls?.map((call) => (
                        <Card key={call.id} className="border border-border">
                          <CardContent className="space-y-3 p-4">
                            <div className="text-sm font-semibold">{call.tool}</div>
                            <div>
                              <div className="text-xs text-muted-foreground mb-1">Arguments</div>
                              <pre className="bg-muted rounded-md p-2 text-xs overflow-x-auto">
                                {JSON.stringify(call.arguments, null, 2)}
                              </pre>
                            </div>
                            <div>
                              <div className="text-xs text-muted-foreground mb-1">Result</div>
                              <pre className="bg-muted rounded-md p-2 text-xs overflow-x-auto">
                                {JSON.stringify(call.output, null, 2)}
                              </pre>
                            </div>
                          </CardContent>
                        </Card>
                      ))
                    )}
                  </>
                )}
                {evidenceModal.type === 'kb' && (
                  <>
                    {selectedMessage?.evidence?.kb_search ? (
                      <EvidenceBlock
                        title="Query"
                        value={selectedMessage.evidence.kb_search.query}
                        results={selectedMessage.evidence.kb_search.results}
                      />
                    ) : (
                      <div className="text-sm text-muted-foreground">No KB search results for this response.</div>
                    )}
                  </>
                )}
                {evidenceModal.type === 'vector' && (
                  <>
                    {selectedMessage?.evidence?.vector_search ? (
                      <EvidenceBlock
                        title="Query"
                        value={selectedMessage.evidence.vector_search.query}
                        results={selectedMessage.evidence.vector_search.results}
                      />
                    ) : (
                      <div className="text-sm text-muted-foreground">No vector search results for this response.</div>
                    )}
                  </>
                )}
              </div>
            </div>
          </div>
        );
      })()}
      {feedbackModal && (() => {
        const draft = getFeedbackDraft(feedbackModal.messageId);
        const submitted = draft.submitted;

        return (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
            <div className="w-full max-w-[520px] overflow-hidden rounded-lg border border-border bg-background shadow-xl">
              <div className="flex items-center justify-between border-b px-4 py-3">
                <div className="text-sm font-semibold">Feedback</div>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Close feedback"
                  onClick={() => setFeedbackModal(null)}
                >
                  <X size={16} />
                </Button>
              </div>
              <div className="p-4 space-y-4">
                {submitted ? (
                  <div className="text-sm text-muted-foreground">
                    Thanks! Your feedback has been recorded.
                  </div>
                ) : (
                  <>
                    <div>
                      <div className="text-xs text-muted-foreground mb-2">Usefulness</div>
                      <div className="flex items-center gap-2">
                        {Array.from({ length: 5 }, (_, index) => {
                          const value = index + 1;
                          const active = value <= draft.rating;
                          return (
                            <button
                              key={value}
                              type="button"
                              className={cn(
                                'h-9 w-9 inline-flex items-center justify-center rounded-md border border-border transition-colors',
                                active ? 'text-amber-400 border-amber-400' : 'text-muted-foreground'
                              )}
                              aria-label={`Rate ${value} star${value === 1 ? '' : 's'}`}
                              onClick={() =>
                                setFeedbackDrafts((prev) => ({
                                  ...prev,
                                  [feedbackModal.messageId]: {
                                    ...draft,
                                    rating: value,
                                  },
                                }))
                              }
                            >
                              <Star size={18} className={active ? 'fill-amber-400' : ''} />
                            </button>
                          );
                        })}
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-muted-foreground mb-2">Notes</div>
                      <textarea
                        rows={4}
                        className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                        placeholder="What was helpful or missing?"
                        value={draft.comment}
                        onChange={(event) =>
                          setFeedbackDrafts((prev) => ({
                            ...prev,
                            [feedbackModal.messageId]: {
                              ...draft,
                              comment: event.target.value,
                            },
                          }))
                        }
                      />
                    </div>
                    {feedbackError && <div className="text-sm text-red-400">{feedbackError}</div>}
                    <div className="flex justify-end gap-2">
                      <Button type="button" variant="secondary" onClick={() => setFeedbackModal(null)}>
                        Cancel
                      </Button>
                      <Button
                        type="button"
                        disabled={feedbackSubmitting || draft.rating === 0}
                        onClick={async () => {
                          if (!sessionId) {
                            setFeedbackError('Session not ready yet. Please try again.');
                            return;
                          }
                          setFeedbackSubmitting(true);
                          setFeedbackError(null);
                          try {
                            await feedbackApi.submit({
                              session_id: sessionId,
                              message_id: feedbackModal.messageId,
                              rating: draft.rating,
                              comment: draft.comment,
                            });
                            setFeedbackDrafts((prev) => ({
                              ...prev,
                              [feedbackModal.messageId]: {
                                ...draft,
                                submitted: true,
                              },
                            }));
                            setFeedbackModal(null);
                          } catch (error) {
                            setFeedbackError(
                              error instanceof Error ? error.message : 'Failed to submit feedback.'
                            );
                          } finally {
                            setFeedbackSubmitting(false);
                          }
                        }}
                      >
                        {feedbackSubmitting ? 'Sending...' : 'Send feedback'}
                      </Button>
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>
        );
      })()}
    </div>
  );
}

function EvidenceBlock({
  title,
  value,
  results,
}: {
  title: string;
  value: string;
  results: EvidenceResult[];
}) {
  return (
    <div className="space-y-3">
      <div>
        <div className="text-xs text-muted-foreground mb-1">{title}</div>
        <div className="text-sm">{value}</div>
      </div>
      <div className="space-y-3">
        {results.map((result, index) => (
          <Card key={`${result.path ?? result.id ?? index}`} className="border border-border">
            <CardContent className="space-y-2 p-4">
              <div className="flex items-center justify-between gap-2">
                <div className="text-sm font-semibold">{result.title || result.path || 'Untitled'}</div>
                {typeof result.score === 'number' && (
                  <div className="text-xs text-muted-foreground">Score: {result.score.toFixed(2)}</div>
                )}
              </div>
              {result.path && <div className="text-xs text-muted-foreground">{result.path}</div>}
              {result.excerpt && (
                <pre className="bg-muted rounded-md p-2 text-xs whitespace-pre-wrap">{result.excerpt}</pre>
              )}
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
