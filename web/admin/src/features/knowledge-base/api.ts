import { apiRequest } from '@/api/client';
import type { KnowledgeBase, KnowledgeBaseInput, KnowledgeBaseListParams, PageResult } from './types';

export const listKnowledgeBases = (params: KnowledgeBaseListParams = {}) =>
  apiRequest<PageResult<KnowledgeBase>>({ url: '/knowledge-bases', params });

export const createKnowledgeBase = (input: KnowledgeBaseInput) =>
  apiRequest<KnowledgeBase>({ url: '/knowledge-bases', method: 'POST', data: input });

export const updateKnowledgeBase = (id: string, input: Partial<KnowledgeBaseInput>) =>
  apiRequest<KnowledgeBase>({ url: `/knowledge-bases/${id}`, method: 'PATCH', data: input });

export const deleteKnowledgeBase = (id: string) =>
  apiRequest<void>({ url: `/knowledge-bases/${id}`, method: 'DELETE' });
