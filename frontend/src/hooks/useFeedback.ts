import { useCallback, useEffect, useMemo, useState } from 'react';
import { feedbackApi } from '../services/api';

const STORAGE_KEY = 'assistant-feedback-v1';

type FeedbackEntry = {
  rating: number;
  comment: string;
  submitted: boolean;
};

type FeedbackState = Record<string, FeedbackEntry>;

export type FeedbackDraft = FeedbackEntry & {
  messageId: string;
};

function loadStored(): FeedbackState {
  if (typeof window === 'undefined') {
    return {};
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw) as FeedbackState;
    if (!parsed || typeof parsed !== 'object') {
      return {};
    }
    return parsed;
  } catch {
    return {};
  }
}

function persistStored(state: FeedbackState) {
  if (typeof window === 'undefined') {
    return;
  }
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // Ignore persistence failures.
  }
}

export function useFeedback(sessionId?: string) {
  const [state, setState] = useState<FeedbackState>({});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setState(loadStored());
  }, []);

  useEffect(() => {
    persistStored(state);
  }, [state]);

  const getDraft = useCallback(
    (messageId: string): FeedbackDraft => {
      const stored = state[messageId];
      return {
        messageId,
        rating: stored?.rating ?? 0,
        comment: stored?.comment ?? '',
        submitted: stored?.submitted ?? false,
      };
    },
    [state]
  );

  const updateDraft = useCallback((messageId: string, next: Partial<FeedbackEntry>) => {
    setState((prev) => ({
      ...prev,
      [messageId]: {
        rating: 0,
        comment: '',
        submitted: false,
        ...prev[messageId],
        ...next,
      },
    }));
  }, []);

  const submitFeedback = useCallback(
    async (messageId: string) => {
      if (!sessionId) {
        setError('Session not ready yet. Please try again.');
        return false;
      }
      const draft = getDraft(messageId);
      if (draft.rating === 0) {
        setError('Please select a rating.');
        return false;
      }

      setSubmitting(true);
      setError(null);
      try {
        await feedbackApi.submit({
          session_id: sessionId,
          message_id: messageId,
          rating: draft.rating,
          comment: draft.comment,
        });
        updateDraft(messageId, { submitted: true });
        return true;
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to submit feedback.');
        return false;
      } finally {
        setSubmitting(false);
      }
    },
    [getDraft, sessionId, updateDraft]
  );

  const submittedCount = useMemo(
    () => Object.values(state).filter((entry) => entry.submitted).length,
    [state]
  );

  return {
    getDraft,
    updateDraft,
    submitFeedback,
    error,
    setError,
    submitting,
    submittedCount,
  };
}
