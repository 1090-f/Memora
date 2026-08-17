import { apiRequest } from '@/api/client';
import type {
  KnowledgeBase,
  KnowledgeBaseDetail,
  KnowledgeBaseDashboard,
  KnowledgeBaseInput,
  KnowledgeBaseListParams,
  KnowledgeBaseUpdateInput,
  PageResult,
  SearchConfig,
  SearchConfigUpdateInput,
} from './types';

export const listKnowledgeBases = (params: KnowledgeBaseListParams = {}) =>
  apiRequest<PageResult<KnowledgeBase>>({ url: '/knowledge-bases', params });

export const getKnowledgeBase = (id: string) =>
  apiRequest<KnowledgeBaseDetail>({ url: `/knowledge-bases/${id}` });

export const getKnowledgeBaseDashboard = (id: string) =>
  apiRequest<KnowledgeBaseDashboard>({ url: `/knowledge-bases/${id}/dashboard` });

export const createKnowledgeBase = (input: KnowledgeBaseInput) =>
  apiRequest<KnowledgeBaseDetail>({ url: '/knowledge-bases', method: 'POST', data: input });

export const updateKnowledgeBase = (id: string, input: KnowledgeBaseUpdateInput) =>
  apiRequest<KnowledgeBaseDetail>({ url: `/knowledge-bases/${id}`, method: 'PATCH', data: input });

export const deleteKnowledgeBase = (id: string) =>
  apiRequest<{ deleted: boolean }>({ url: `/knowledge-bases/${id}`, method: 'DELETE' });

export const getSearchConfig = (kbId: string) =>
  apiRequest<SearchConfig>({ url: `/knowledge-bases/${kbId}/search-config` });

export const updateSearchConfig = (kbId: string, input: SearchConfigUpdateInput) =>
  apiRequest<SearchConfig>({ url: `/knowledge-bases/${kbId}/search-config`, method: 'PUT', data: input });
