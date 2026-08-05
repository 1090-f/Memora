import { afterEach, describe, expect, it, vi } from 'vitest';
import { AUTH_STORAGE_KEY } from '@/features/auth/session';
import { AppError } from './errors';
import { readSseStream, type SseEvent } from './sse';

const streamResponse = (source: string, cuts: number[] = []) => {
  const bytes = new TextEncoder().encode(source);
  const boundaries = [...cuts, bytes.length];
  let start = 0;
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      for (const end of boundaries) {
        controller.enqueue(bytes.slice(start, end));
        start = end;
      }
      controller.close();
    },
  });
  return new Response(body, {
    status: 200,
    headers: { 'content-type': 'text/event-stream' },
  });
};

describe('readSseStream', () => {
  afterEach(() => {
    sessionStorage.clear();
    vi.unstubAllGlobals();
  });

  it('parses split UTF-8 frames, ignores duplicate sequences, and stops at completion', async () => {
    sessionStorage.setItem(
      AUTH_STORAGE_KEY,
      JSON.stringify({ access_token: 'stream-token', expires_at: Date.now() + 60_000 }),
    );
    const source = [
      'id: 1\nevent: answer.delta\ndata: {"sequence":1,"text":"你好"}\n\n',
      'id: duplicate\nevent: answer.delta\ndata: {"sequence":1,"text":"重复"}\n\n',
      'id: 2\nevent: completed\ndata: {"sequence":2,"status":"completed"}\n\n',
      'id: 3\nevent: answer.delta\ndata: {"sequence":3,"text":"不应读取"}\n\n',
    ].join('');
    const fetchMock = vi.fn().mockResolvedValue(streamResponse(source, [3, 17, 41, 66]));
    vi.stubGlobal('fetch', fetchMock);
    const events: SseEvent[] = [];

    await readSseStream('/api/v1/agent-runs/run-1/events', {
      signal: new AbortController().signal,
      afterSequence: 0,
      onEvent: (event) => events.push(event),
    });

    expect(events).toEqual([
      { id: '1', event: 'answer.delta', data: { sequence: 1, text: '你好' }, sequence: 1 },
      { id: '2', event: 'completed', data: { sequence: 2, status: 'completed' }, sequence: 2 },
    ]);
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/agent-runs/run-1/events?after_sequence=0',
      expect.objectContaining({
        headers: expect.objectContaining({
          Accept: 'text/event-stream',
          Authorization: 'Bearer stream-token',
        }),
      }),
    );
  });

  it('propagates a protocol error with its safe message', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        streamResponse('event: error\ndata: {"sequence":4,"message":"模型服务不可用"}\n\n'),
      ),
    );

    await expect(
      readSseStream('/events', {
        signal: new AbortController().signal,
        onEvent: () => undefined,
      }),
    ).rejects.toMatchObject({
      code: 'SSE_PROTOCOL_ERROR',
      message: '模型服务不可用',
    } satisfies Partial<AppError>);
  });

  it('treats AbortError as an intentional cancellation', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockRejectedValue(new DOMException('cancelled', 'AbortError')),
    );

    await expect(
      readSseStream('/events', {
        signal: new AbortController().signal,
        onEvent: () => undefined,
      }),
    ).resolves.toBeUndefined();
  });
});
