import { apiRequest } from '@/api/client';
import type { SearchInput, SearchResponse, SearchTestResponse } from './types';

export const runSearch = (kbId: string, input: SearchInput) =>
  apiRequest<SearchResponse>({ url: `/knowledge-bases/${kbId}/search`, method: 'POST', data: input });

export const runSearchTest = (kbId: string, input: SearchInput) =>
  apiRequest<SearchTestResponse>({ url: `/knowledge-bases/${kbId}/search/test`, method: 'POST', data: input });
