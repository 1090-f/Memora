import { apiRequest } from '@/api/client';
import type { PageResult } from '@/features/knowledge-base/types';
import type { Memory, MemoryStatus } from './types';

export const listMemories = (params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<Memory>>({ url: '/memories', params });
export const getMemory = (id: string) => apiRequest<Memory>({ url: `/memories/${id}` });
export const updateMemoryStatus = (id: string, status: Exclude<MemoryStatus, 'deleted'>) =>
  apiRequest<Memory>({ url: `/memories/${id}/status`, method: 'PATCH', data: { status } });
export const deleteMemory = (id: string) => apiRequest<void>({ url: `/memories/${id}`, method: 'DELETE' });
