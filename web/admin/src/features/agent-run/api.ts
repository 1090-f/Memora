import { apiRequest } from '@/api/client';
import type { PageResult } from '@/features/knowledge-base/types';
import type {
  AgentRun,
  AgentRunListItem,
  AgentToolCall,
  TraceSpan,
  CreateAgentRunResponse,
} from './types';

export const listAgentRuns = (params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<AgentRunListItem>>({ url: '/agent/runs', params });
export const getAgentRun = (id: string) =>
  apiRequest<AgentRun>({ url: `/agent/runs/${id}` });
export const getAgentRunToolCalls = (runId: string) =>
  apiRequest<AgentToolCall[]>({ url: `/agent/runs/${runId}/tool-calls` });
export const getAgentRunTrace = (runId: string) =>
  apiRequest<TraceSpan[]>({ url: `/agent/runs/${runId}/trace` });
export const createAgentRun = (input: {
  knowledge_base_id: string;
  conversation_id: string;
  query: string;
}) =>
  apiRequest<CreateAgentRunResponse>({
    url: '/agent/runs',
    method: 'POST',
    data: input,
  });
export const cancelAgentRun = (id: string) =>
  apiRequest<{ cancelled: boolean }>({
    url: `/agent/runs/${id}/cancel`,
    method: 'POST',
  });
export const retryAgentRun = (id: string) =>
  apiRequest<{ new_run_id: string; status: 'queued' }>({
    url: `/agent/runs/${id}/retry`,
    method: 'POST',
  });
