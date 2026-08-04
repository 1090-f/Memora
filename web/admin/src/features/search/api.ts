import { apiRequest } from '@/api/client';
import type { SearchInput, SearchTestResponse } from './types';
export const runSearchTest = (kbId: string, input: SearchInput) =>
  apiRequest<SearchTestResponse>({ url: `/knowledge-bases/${kbId}/search/test`, method: 'POST', data: input });
