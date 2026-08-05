import type { PageQuery } from '@/types/common'

export interface DirectoryNode {
  id: string
  knowledge_base_id: string
  parent_id: string | null
  name: string
  depth: number
  sort_order: number
  children: DirectoryNode[]
}

export interface CreateDirectoryRequest {
  name: string
  parent_id?: string | null
  sort_order?: number
}

export interface UpdateDirectoryRequest {
  name?: string
  parent_id?: string | null
  sort_order?: number
}

export interface DocumentListItem {
  id: string
  knowledge_base_id: string
  directory_id: string | null
  title: string
  source_type: 'manual' | 'file' | 'url'
  source_url: string | null
  processing_status: ProcessingStatus
  content_version: number
  active_index_version: number | null
  created_at: string
  updated_at: string
}

export interface DocumentDetail extends DocumentListItem {
  content: string
}

export type ProcessingStatus =
  | 'pending'
  | 'parsing'
  | 'cleaning'
  | 'chunking'
  | 'embedding'
  | 'keyword_indexing'
  | 'succeeded'
  | 'failed'

export interface ProcessingState {
  document_id: string
  processing_status: ProcessingStatus
  current_index_version: number
  active_index_version: number
  failure_step: string | null
  failure_reason: string | null
}

export interface DocumentListQuery extends PageQuery {
  directory_id?: string
  processing_status?: ProcessingStatus
  source_type?: string
}

export const documentKeys = {
  all: ['documents'] as const,
  list: (kbId: string, query?: DocumentListQuery) => ['documents', 'list', kbId, query] as const,
  detail: (id: string) => ['documents', 'detail', id] as const,
  processing: (id: string) => ['documents', 'processing', id] as const,
  directories: (kbId: string) => ['directories', kbId] as const,
}
