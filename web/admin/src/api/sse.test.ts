import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AUTH_STORAGE_KEY } from '@/features/auth/session';
import { readSseStream, type SseEvent } from './sse';

const responseWithFrames = (...chunks: string[]): Response => {
  const encoder = new TextEncoder();
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      chunks.forEach((chunk) => controller.enqueue(encoder.encode(chunk)));
      controller.close();
    },
  });
  return { ok: true, status: 200, body, headers: new Headers() } as Response;
};

describe('readSseStream', () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.unstubAllGlobals();
  });

  it('continues from after_sequence, drops duplicates and ignores complete control frames', async () => {
    sessionStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify({ access_token: 'token-1', expires_at: Date.now() + 60_000 }));
    const fetchMock = vi.fn().mockResolvedValue(responseWithFrames(
      'event: agent.run.started\ndata: {"sequence":1,"label":"duplicate"}\n\n',
      'event: agent.stage.updated\ndata: {"sequence":2,"label":"阶段完成"}\n\n',
      'event: complete\ndata: {}\n\n',
    ));
    vi.stubGlobal('fetch', fetchMock);
    const received: SseEvent[] = [];

    await readSseStream('/api/v1/agent/runs/run-1/events', {
      signal: new AbortController().signal,
      afterSequence: 1,
      onEvent: (item) => received.push(item),
    });

    expect(fetchMock).toHaveBeenCalledOnce();
    const [url, options] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/v1/agent/runs/run-1/events?after_sequence=1');
    expect(options.headers).toMatchObject({ Authorization: 'Bearer token-1' });
    expect(received).toHaveLength(1);
    expect(received[0]).toMatchObject({ event: 'agent.stage.updated', sequence: 2, data: { label: '阶段完成' } });
  });

  it('parses a UTF-8 frame split across network chunks', async () => {
    const encoded = new TextEncoder().encode('event: message\ndata: {"sequence":1,"text":"中文"}\n\n');
    const split = encoded.findIndex((byte) => byte > 0x7f) + 1;
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(encoded.slice(0, split));
        controller.enqueue(encoded.slice(split));
        controller.close();
      },
    });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, body, headers: new Headers() } as Response));
    const received: SseEvent[] = [];

    await readSseStream('/events', {
      signal: new AbortController().signal,
      onEvent: (item) => received.push(item),
    });

    expect(received[0]?.data).toEqual({ sequence: 1, text: '中文' });
  });
});
