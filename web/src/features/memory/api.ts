import { request } from '@/api/client'
import { toSearchParams } from '@/api/pagination'
import type { PageData } from '@/types/common'
import type { Memory, MemoryListQuery } from './types'

export function listMemories(query?: MemoryListQuery): Promise<PageData<Memory>> {
  const params = toSearchParams(query)
  if (query?.memory_type) params.set('memory_type', query.memory_type)
  if (query?.scope_type) params.set('scope_type', query.scope_type)
  if (query?.scope_id) params.set('scope_id', query.scope_id)
  if (query?.status) params.set('status', query.status)
  return request<PageData<Memory>>(`/memories?${params.toString()}`)
}

export function getMemory(id: string): Promise<Memory> {
  return request<Memory>(`/memories/${id}`)
}

export function updateMemoryStatus(id: string, status: 'active' | 'inactive'): Promise<void> {
  return request<void>(`/memories/${id}/status`, {
    method: 'PATCH',
    body: { status },
  })
}

export function deleteMemory(id: string): Promise<void> {
  return request<void>(`/memories/${id}`, {
    method: 'DELETE',
  })
}
