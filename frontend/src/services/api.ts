import type { ChatRequest, StreamChunk } from '../types';

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
