import { apiRequest } from '@/api/client';
import type { PageResult } from '@/features/knowledge-base/types';
import type { AgentRun } from './types';

export const listAgentRuns = (params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<AgentRun>>({ url: '/agent-runs', params });
export const getAgentRun = (id: string) => apiRequest<AgentRun>({ url: `/agent-runs/${id}` });
export const cancelAgentRun = (id: string) =>
  apiRequest<{ run_id: string; status: 'cancelled' }>({ url: `/agent-runs/${id}/cancel`, method: 'POST' });
export const retryAgentRun = (id: string) =>
  apiRequest<{ new_run_id: string; retry_of_run_id: string; status: 'queued' }>({ url: `/agent-runs/${id}/retry`, method: 'POST' });
