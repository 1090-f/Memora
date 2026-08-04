import { readSseStream } from '@/api/sse';
import type { AgentEvent, AgentEventType } from '@/features/agent-run/types';

export const streamAgentEvents = (
  url: string,
  options: { signal: AbortSignal; afterSequence?: number; onEvent: (event: AgentEvent) => void },
) => readSseStream(url, {
  signal: options.signal,
  afterSequence: options.afterSequence,
  onEvent: (event) => {
    const body = event.data as Omit<AgentEvent, 'type'>;
    options.onEvent({ ...body, type: event.event as AgentEventType });
  },
});
