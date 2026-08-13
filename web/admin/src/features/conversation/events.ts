import { readSseStream } from '@/api/sse';
import type { AgentEvent, AgentEventType } from '@/features/agent-run/types';

export const streamAgentEvents = (
  url: string,
  options: { signal: AbortSignal; afterSequence?: number; onEvent: (event: AgentEvent) => void },
) => readSseStream(url, {
  signal: options.signal,
  afterSequence: options.afterSequence,
  onEvent: (event) => {
    const body = event.data as {
      run_id: string;
      sequence: number;
      timestamp: string;
      event_type: AgentEventType;
      data: Record<string, unknown>;
    };
    options.onEvent({
      run_id: body.run_id,
      sequence: body.sequence,
      timestamp: body.timestamp,
      type: body.event_type,
      payload: body.data || {},
    });
  },
});
