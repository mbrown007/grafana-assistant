import type { ChatRequest, CurrentUser, HistoryDetail, HistorySession, StreamChunk } from '../types';

async function* streamResponse(response: Response): AsyncGenerator<StreamChunk> {
  if (!response.body) {
    throw new Error('Streaming response not available');
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder('utf-8');
  let buffer = '';

  while (true) {
    const { value, done } = await reader.read();
    if (done) {
      break;
    }

    buffer += decoder.decode(value, { stream: true });
    let boundaryIndex = buffer.indexOf('\n\n');

    while (boundaryIndex !== -1) {
      const rawEvent = buffer.slice(0, boundaryIndex);
      buffer = buffer.slice(boundaryIndex + 2);
      const lines = rawEvent.split('\n');

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed.startsWith('data:')) {
          continue;
        }
        const json = trimmed.replace(/^data:\s*/, '');
        if (!json) {
          continue;
        }
        try {
          const chunk = JSON.parse(json) as StreamChunk;
          yield chunk;
        } catch (error) {
          console.error('Failed to parse SSE chunk', error, json);
        }
      }

      boundaryIndex = buffer.indexOf('\n\n');
    }
  }
}

export const chatApi = {
  async *stream(payload: ChatRequest): AsyncGenerator<StreamChunk> {
    const response = await fetch('/api/chat', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      const message = await response.text();
      throw new Error(message || `Request failed (${response.status})`);
    }

    yield* streamResponse(response);
  },
};

export const historyApi = {
  async list(): Promise<HistorySession[]> {
    const response = await fetch('/api/history');
    if (!response.ok) {
      const message = await response.text();
      throw new Error(message || `Request failed (${response.status})`);
    }
    return (await response.json()) as HistorySession[];
  },
  async get(id: string): Promise<HistoryDetail> {
    const response = await fetch(`/api/history/${id}`);
    if (!response.ok) {
      const message = await response.text();
      throw new Error(message || `Request failed (${response.status})`);
    }
    return (await response.json()) as HistoryDetail;
  },
  async remove(id: string): Promise<void> {
    const response = await fetch(`/api/history/${id}`, {
      method: 'DELETE',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
    if (!response.ok) {
      const message = await response.text();
      throw new Error(message || `Request failed (${response.status})`);
    }
  },
};

export const userApi = {
  async get(): Promise<CurrentUser> {
    const response = await fetch('/api/me');
    if (!response.ok) {
      const message = await response.text();
      throw new Error(message || `Request failed (${response.status})`);
    }
    const contentType = response.headers.get('content-type') || '';
    if (!contentType.includes('application/json')) {
      const message = await response.text();
      throw new Error(message || 'Unexpected response while checking Grafana login.');
    }
    return (await response.json()) as CurrentUser;
  },
};
