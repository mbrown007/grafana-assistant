import { chatApi, historyApi, userApi } from '../api';

// Helper to create a ReadableStream from SSE text.
function sseStream(events: string): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return new ReadableStream({
    start(controller) {
      controller.enqueue(encoder.encode(events));
      controller.close();
    },
  });
}

describe('chatApi.stream', () => {
  it('parses SSE chunks from a streaming response', async () => {
    const sse = 'data: {"type":"start","session_id":"s1"}\n\ndata: {"type":"token","message":"Hello"}\n\ndata: {"type":"done"}\n\n';
    const mockResponse = new Response(sseStream(sse), {
      status: 200,
      headers: { 'Content-Type': 'text/event-stream' },
    });
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(mockResponse);

    const chunks = [];
    for await (const chunk of chatApi.stream({ message: 'hi' })) {
      chunks.push(chunk);
    }

    expect(chunks).toHaveLength(3);
    expect(chunks[0]).toEqual({ type: 'start', session_id: 's1' });
    expect(chunks[1]).toEqual({ type: 'token', message: 'Hello' });
    expect(chunks[2]).toEqual({ type: 'done' });
  });

  it('throws on non-OK responses', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response('unauthorized', { status: 401 })
    );

    await expect(async () => {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      for await (const _ of chatApi.stream({ message: 'hi' })) {
        // consume
      }
    }).rejects.toThrow('unauthorized');
  });
});

describe('historyApi', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('list returns sessions', async () => {
    const sessions = [{ id: 's1', title: 'Chat 1', created_at: '2025-01-01', updated_at: '2025-01-01' }];
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify(sessions), { status: 200, headers: { 'Content-Type': 'application/json' } })
    );

    const result = await historyApi.list();
    expect(result).toEqual(sessions);
  });

  it('get returns session detail', async () => {
    const detail = {
      session: { id: 's1', title: 'Chat 1', created_at: '2025-01-01', updated_at: '2025-01-01' },
      messages: [{ id: 'm1', role: 'user', content: 'hi', created_at: '2025-01-01' }],
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify(detail), { status: 200, headers: { 'Content-Type': 'application/json' } })
    );

    const result = await historyApi.get('s1');
    expect(result.session.id).toBe('s1');
    expect(result.messages).toHaveLength(1);
  });

  it('remove calls DELETE', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(null, { status: 204 })
    );

    await historyApi.remove('s1');
    expect(fetchSpy).toHaveBeenCalledWith('/api/history/s1', expect.objectContaining({ method: 'DELETE' }));
  });
});

describe('userApi', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns current user', async () => {
    const user = { id: 1, login: 'admin', name: 'Admin', org_id: 1 };
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify(user), { status: 200, headers: { 'Content-Type': 'application/json' } })
    );

    const result = await userApi.get();
    expect(result.login).toBe('admin');
  });

  it('throws on non-JSON response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response('<html>login page</html>', { status: 200, headers: { 'Content-Type': 'text/html' } })
    );

    await expect(userApi.get()).rejects.toThrow();
  });
});
