import type { PageQuery } from '@/types/common'

export type MemoryType = 'preference' | 'project' | 'decision' | 'goal' | 'fact' | 'progress'
export type MemoryScope = 'user' | 'knowledge_base'
export type MemoryStatus = 'active' | 'inactive' | 'deleted'

export interface Memory {
  id: string
  memory_type: MemoryType
  scope_type: MemoryScope
  scope_id: string | null
  content: string
  summary: string | null
  importance: number | null
  source_conversation_id: string | null
  source_message_id: string | null
  status: MemoryStatus
  created_at: string
  updated_at: string
  last_accessed_at: string | null
}

export interface MemoryListQuery extends PageQuery {
  memory_type?: MemoryType
  scope_type?: MemoryScope
  scope_id?: string
  status?: MemoryStatus
}

export const memoryKeys = {
  all: ['memories'] as const,
  list: (query?: MemoryListQuery) => ['memories', 'list', query] as const,
  detail: (id: string) => ['memories', 'detail', id] as const,
}
