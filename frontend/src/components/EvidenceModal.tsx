import React from 'react';
import { X } from 'lucide-react';
import type { EvidencePayload, EvidenceResult, Message } from '../types';
import { Button } from './ui/button';
import { Card, CardContent } from './ui/card';

interface EvidenceModalProps {
  message: Message | undefined;
  type: 'tool' | 'kb' | 'vector';
  onClose: () => void;
}

export function EvidenceModal({ message, type, onClose }: EvidenceModalProps) {
  const title = type === 'tool' ? 'Tool calls' : type === 'kb' ? 'KB search' : 'Vector search';

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
        </div>
      </div>
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
