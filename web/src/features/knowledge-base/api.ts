import { request } from '@/api/client'
import { toSearchParams } from '@/api/pagination'
import type { PageData } from '@/types/common'
import type { KnowledgeBase, CreateKnowledgeBaseRequest, UpdateKnowledgeBaseRequest } from './types'

export function listKnowledgeBases(query?: { page?: number; page_size?: number; keyword?: string; sort?: string }): Promise<PageData<KnowledgeBase>> {
  const params = toSearchParams(query)
  return request<PageData<KnowledgeBase>>(`/knowledge-bases?${params.toString()}`)
}

export function getKnowledgeBase(id: string): Promise<KnowledgeBase> {
  return request<KnowledgeBase>(`/knowledge-bases/${id}`)
}

export function createKnowledgeBase(data: CreateKnowledgeBaseRequest): Promise<KnowledgeBase> {
  return request<KnowledgeBase>('/knowledge-bases', {
    method: 'POST',
    body: data,
  })
}

export function updateKnowledgeBase(id: string, data: UpdateKnowledgeBaseRequest): Promise<KnowledgeBase> {
  return request<KnowledgeBase>(`/knowledge-bases/${id}`, {
    method: 'PATCH',
    body: data,
  })
}

export function deleteKnowledgeBase(id: string): Promise<void> {
  return request<void>(`/knowledge-bases/${id}`, {
    method: 'DELETE',
  })
}
