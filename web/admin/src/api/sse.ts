import { readStoredSession } from '@/features/auth/session';
import { AppError } from './errors';

export interface SseEvent<T = unknown> {
  id?: string;
  event: string;
  data: T;
  sequence?: number;
}

export interface SseOptions {
  signal: AbortSignal;
  afterSequence?: number;
  onEvent: (event: SseEvent) => void;
  /** SSE 流最大等待时间（毫秒），超时后静默关闭流。默认 0 = 不超时。 */
  timeout?: number;
}

const terminalEvents = new Set([
  'done',
  'completed',
  'error',
  'agent.run.completed',
  'agent.run.failed',
  'agent.run.cancelled',
]);

const parseData = (source: string): unknown => {
  try {
    return JSON.parse(source) as unknown;
  } catch {
    return source;
  }
};

const getSequence = (data: unknown): number | undefined => {
  if (!data || typeof data !== 'object') return undefined;
  const sequence = (data as { sequence?: unknown }).sequence;
  return typeof sequence === 'number' ? sequence : undefined;
};

const getProtocolMessage = (data: unknown): string => {
  if (data && typeof data === 'object') {
    const message = (data as { message?: unknown }).message;
    if (typeof message === 'string' && message.length > 0) return message;
  }
  return '流式响应失败';
};

const parseFrame = (frame: string): SseEvent | null => {
  let id: string | undefined;
  let event = 'message';
  const dataLines: string[] = [];

  for (const line of frame.split('\n')) {
    if (line.startsWith(':')) continue;
    const separator = line.indexOf(':');
    const field = separator === -1 ? line : line.slice(0, separator);
    const raw = separator === -1 ? '' : line.slice(separator + 1);
    const value = raw.startsWith(' ') ? raw.slice(1) : raw;

    if (field === 'id') id = value;
    if (field === 'event') event = value || 'message';
    if (field === 'data') dataLines.push(value);
  }

  if (dataLines.length === 0) return null;
  const data = parseData(dataLines.join('\n'));
  return { id, event, data, sequence: getSequence(data) };
};

const buildRequestUrl = (url: string, afterSequence?: number): string => {
  const absolute = /^https?:\/\//i.test(url);
  const parsed = new URL(url, window.location.origin);
  if (afterSequence !== undefined) {
    parsed.searchParams.set('after_sequence', String(afterSequence));
  }
  return absolute ? parsed.toString() : `${parsed.pathname}${parsed.search}`;
};

export async function readSseStream(
  url: string,
  options: SseOptions,
): Promise<void> {
  const session = readStoredSession();
  const headers: Record<string, string> = { Accept: 'text/event-stream' };
  if (session) headers.Authorization = `Bearer ${session.access_token}`;

  // 超时保护：创建组合 AbortSignal（用户取消 + 超时）
  let timeoutId: ReturnType<typeof setTimeout> | undefined;
  let combinedSignal = options.signal;
  if (options.timeout && options.timeout > 0) {
    const timeoutController = new AbortController();
    timeoutId = setTimeout(() => timeoutController.abort(), options.timeout);
    try {
      combinedSignal = AbortSignal.any([options.signal, timeoutController.signal]);
    } catch {
      combinedSignal = options.signal;
    }
  }

  try {
    const response = await fetch(buildRequestUrl(url, options.afterSequence), {
      method: 'GET',
      headers,
      signal: combinedSignal,
    });

    if (!response.ok || !response.body) {
      throw new AppError(
        'SSE_TRANSPORT_ERROR',
        '无法建立流式连接',
        response.status,
        undefined,
        response.headers.get('x-request-id') || '',
      );
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let highestSequence = options.afterSequence ?? -1;

    while (true) {
      const { done, value } = await reader.read();
      buffer += decoder.decode(value, { stream: !done }).replace(/\r\n/g, '\n');

      let boundary = buffer.indexOf('\n\n');
      while (boundary !== -1) {
        const frameText = buffer.slice(0, boundary);
        buffer = buffer.slice(boundary + 2);
        boundary = buffer.indexOf('\n\n');

        const event = parseFrame(frameText);
        if (!event) continue;
        if (event.sequence !== undefined && event.sequence <= highestSequence) continue;
        if (event.sequence !== undefined) highestSequence = event.sequence;

        options.onEvent(event);

        if (event.event === 'error') {
          throw new AppError(
            'SSE_PROTOCOL_ERROR',
            getProtocolMessage(event.data),
            200,
            event.data,
            event.id || '',
          );
        }
        if (terminalEvents.has(event.event)) {
          await reader.cancel();
          return;
        }
      }

      if (done) return;
    }
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return;
    throw error;
  } finally {
    if (timeoutId !== undefined) clearTimeout(timeoutId);
  }
}
