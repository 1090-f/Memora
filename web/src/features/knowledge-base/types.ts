import type { PageQuery } from '@/types/common'

export interface KnowledgeBase {
  id: string
  name: string
  description?: string | null
  icon?: string
  document_count?: number
  agent_enabled?: boolean
  network_enabled?: boolean
  created_at: string
  updated_at: string
}

export interface CreateKnowledgeBaseRequest {
  name: string
  description?: string
  icon?: string
  default_language?: string
  qa_enabled?: boolean
  agent_enabled?: boolean
  network_enabled?: boolean
  default_chat_model_id?: string
  default_embedding_model_id?: string
  default_reranker_model_id?: string
}

export interface UpdateKnowledgeBaseRequest {
  name?: string
  description?: string
  icon?: string
  default_language?: string
  qa_enabled?: boolean
  agent_enabled?: boolean
  network_enabled?: boolean
  default_chat_model_id?: string
  default_embedding_model_id?: string
  default_reranker_model_id?: string
}

export const knowledgeBaseKeys = {
  all: ['knowledge-bases'] as const,
  list: (query?: PageQuery) => ['knowledge-bases', 'list', query] as const,
  detail: (id: string) => ['knowledge-bases', 'detail', id] as const,
}
