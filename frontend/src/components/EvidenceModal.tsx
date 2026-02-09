import React from 'react';
import { X } from 'lucide-react';
import type { EvidenceResult, Message, TimelineStep } from '../types';
import { Button } from './ui/button';
import { Card, CardContent } from './ui/card';

interface EvidenceModalProps {
  message: Message | undefined;
  type: 'tool' | 'kb' | 'vector' | 'timeline';
  onClose: () => void;
}

export function EvidenceModal({ message, type, onClose }: EvidenceModalProps) {
  const title =
    type === 'tool'
      ? 'Tool calls'
      : type === 'kb'
        ? 'KB search'
        : type === 'vector'
          ? 'Vector search'
          : 'Response timeline';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-[720px] max-h-[80vh] overflow-hidden rounded-lg border border-border bg-background shadow-xl">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div className="text-sm font-semibold">{title}</div>
          <Button type="button" variant="ghost" size="icon" aria-label="Close evidence" onClick={onClose}>
            <X size={16} />
          </Button>
        </div>
        <div className="p-4 overflow-y-auto max-h-[72vh] space-y-4">
          {type === 'tool' && (
            <>
              {(message?.toolCalls || []).length === 0 ? (
                <div className="text-sm text-muted-foreground">No tool calls recorded for this response.</div>
              ) : (
                message?.toolCalls?.map((call) => (
                  <Card key={call.id} className="border border-border">
                    <CardContent className="space-y-3 p-4">
                      <div className="text-sm font-semibold">{call.tool}</div>
                      {call.reason && (
                        <div>
                          <div className="text-xs text-muted-foreground mb-1">Why this tool</div>
                          <div className="text-xs whitespace-pre-wrap">{call.reason}</div>
                        </div>
                      )}
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
          {type === 'kb' && (
            <>
              {message?.evidence?.kb_search ? (
                <EvidenceBlock
                  title="Query"
                  value={message.evidence.kb_search.query}
                  results={message.evidence.kb_search.results}
                />
              ) : (
                <div className="text-sm text-muted-foreground">No KB search results for this response.</div>
              )}
            </>
          )}
          {type === 'vector' && (
            <>
              {message?.evidence?.vector_search ? (
                <EvidenceBlock
                  title="Query"
                  value={message.evidence.vector_search.query}
                  results={message.evidence.vector_search.results}
                />
              ) : (
                <div className="text-sm text-muted-foreground">No vector search results for this response.</div>
              )}
            </>
          )}
          {type === 'timeline' && (
            <>
              {(message?.timeline || []).length === 0 ? (
                <div className="text-sm text-muted-foreground">No timeline events recorded for this response.</div>
              ) : (
                <div className="space-y-3">
                  {[...(message?.timeline || [])]
                    .sort((a, b) => a.order - b.order)
                    .map((step) => (
                      <Card key={step.id} className="border border-border">
                        <CardContent className="space-y-2 p-4">
                          <div className="flex items-center justify-between gap-2">
                            <div className="text-sm font-semibold">{step.title}</div>
                            <div className="text-xs text-muted-foreground">{formatTimelineTimestamp(step.timestamp)}</div>
                          </div>
                          <div className="flex items-center gap-2">
                            <span
                              className={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${timelineStatusClass(
                                step.status
                              )}`}
                            >
                              {timelineKindLabel(step)}
                            </span>
                          </div>
                          {step.detail && <div className="text-xs text-muted-foreground whitespace-pre-wrap">{step.detail}</div>}
                        </CardContent>
                      </Card>
                    ))}
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function timelineKindLabel(step: TimelineStep): string {
  switch (step.kind) {
    case 'start':
      return 'Start';
    case 'tool_call':
      return 'Tool call';
    case 'retry':
      return 'Retry';
    case 'tool_result':
      return 'Tool result';
    case 'final_answer':
      return 'Final answer';
    case 'error':
      return 'Error';
    default:
      return 'Event';
  }
}

function timelineStatusClass(status: TimelineStep['status']): string {
  switch (status) {
    case 'ok':
      return 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300';
    case 'warning':
      return 'bg-amber-500/15 text-amber-700 dark:text-amber-300';
    case 'error':
      return 'bg-red-500/15 text-red-700 dark:text-red-300';
    default:
      return 'bg-muted text-muted-foreground';
  }
}

function formatTimelineTimestamp(timestamp: string): string {
  const parsed = new Date(timestamp);
  if (Number.isNaN(parsed.getTime())) {
    return timestamp;
  }
  return parsed.toLocaleTimeString();
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
