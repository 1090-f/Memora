import { request } from '@/api/client'
import type { SearchTestRequest, SearchTestResponse, SearchRequest, SearchResponse } from './types'

export function searchTest(kbId: string, data: SearchTestRequest): Promise<SearchTestResponse> {
  return request<SearchTestResponse>(`/knowledge-bases/${kbId}/search/test`, {
    method: 'POST',
    body: data,
  })
}

export function search(kbId: string, data: SearchRequest): Promise<SearchResponse> {
  return request<SearchResponse>(`/knowledge-bases/${kbId}/search`, {
    method: 'POST',
    body: data,
  })
}
