import React from 'react';
import { Check, Star, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from './ui/button';

interface FeedbackModalProps {
  messageId: string;
  rating: number;
  comment: string;
  submitted: boolean;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onRate: (value: number) => void;
  onComment: (value: string) => void;
  onSubmit: () => void;
}

export function FeedbackModal({
  messageId,
  rating,
  comment,
  submitted,
  submitting,
  error,
  onClose,
  onRate,
  onComment,
  onSubmit,
}: FeedbackModalProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-[520px] overflow-hidden rounded-lg border border-border bg-background shadow-xl">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div className="text-sm font-semibold">Feedback</div>
          <Button type="button" variant="ghost" size="icon" aria-label="Close feedback" onClick={onClose}>
            <X size={16} />
          </Button>
        </div>
        <div className="p-4 space-y-4">
          {submitted ? (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <Check size={16} className="text-emerald-500" /> Thanks! Your feedback has been recorded.
            </div>
          ) : (
            <>
              <div>
                <div className="text-xs text-muted-foreground mb-2">Usefulness</div>
                <div className="flex items-center gap-2">
                  {Array.from({ length: 5 }, (_, index) => {
                    const value = index + 1;
                    const active = value <= rating;
                    return (
                      <button
                        key={`${messageId}-${value}`}
                        type="button"
                        className={cn(
                          'h-9 w-9 inline-flex items-center justify-center rounded-md border border-border transition-colors',
                          active ? 'text-amber-400 border-amber-400' : 'text-muted-foreground'
                        )}
                        aria-label={`Rate ${value} star${value === 1 ? '' : 's'}`}
                        onClick={() => onRate(value)}
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
                  value={comment}
                  onChange={(event) => onComment(event.target.value)}
                />
              </div>
              {error && <div className="text-sm text-red-400">{error}</div>}
              <div className="flex justify-end gap-2">
                <Button type="button" variant="secondary" onClick={onClose}>
                  Cancel
                </Button>
                <Button type="button" disabled={submitting || rating === 0} onClick={onSubmit}>
                  {submitting ? 'Sending...' : 'Send feedback'}
                </Button>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
