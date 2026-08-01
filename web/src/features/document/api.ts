import { request } from '@/api/client'
import { toSearchParams } from '@/api/pagination'
import type { PageData } from '@/types/common'
import type {
  DirectoryNode,
  CreateDirectoryRequest,
  UpdateDirectoryRequest,
  DocumentListItem,
  DocumentDetail,
  ProcessingState,
  DocumentListQuery,
} from './types'

// Directory APIs
export function getDirectoryTree(kbId: string): Promise<DirectoryNode[]> {
  return request<DirectoryNode[]>(`/knowledge-bases/${kbId}/directories/tree`)
}

export function createDirectory(kbId: string, data: CreateDirectoryRequest): Promise<DirectoryNode> {
  return request<DirectoryNode>(`/knowledge-bases/${kbId}/directories`, {
    method: 'POST',
    body: data,
  })
}

export function updateDirectory(id: string, data: UpdateDirectoryRequest): Promise<DirectoryNode> {
  return request<DirectoryNode>(`/directories/${id}`, {
    method: 'PATCH',
    body: data,
  })
}

export function deleteDirectory(id: string): Promise<void> {
  return request<void>(`/directories/${id}`, {
    method: 'DELETE',
  })
}

// Document APIs
export function listDocuments(kbId: string, query?: DocumentListQuery): Promise<PageData<DocumentListItem>> {
  const params = toSearchParams(query)
  if (query?.directory_id) params.set('directory_id', query.directory_id)
  if (query?.processing_status) params.set('processing_status', query.processing_status)
  if (query?.source_type) params.set('source_type', query.source_type)
  return request<PageData<DocumentListItem>>(`/knowledge-bases/${kbId}/documents?${params.toString()}`)
}

export function getDocument(id: string): Promise<DocumentDetail> {
  return request<DocumentDetail>(`/documents/${id}`)
}

export function deleteDocument(id: string): Promise<void> {
  return request<void>(`/documents/${id}`, {
    method: 'DELETE',
  })
}

export function getProcessingState(id: string): Promise<ProcessingState> {
  return request<ProcessingState>(`/documents/${id}/processing`)
}

export function retryProcessing(id: string): Promise<void> {
  return request<void>(`/documents/${id}/retry-processing`, {
    method: 'POST',
  })
}

export function reindexDocument(id: string, reason?: string): Promise<{ document_id: string; new_index_version: number; active_index_version: number; status: string }> {
  return request(`/documents/${id}/reindex`, {
    method: 'POST',
    body: { reason: reason || 'manual' },
  })
}

export interface IndexVersion {
  id: string
  version: number
  status: 'processing' | 'active' | 'inactive' | 'failed'
  created_at: string
}

export function getIndexVersions(id: string): Promise<IndexVersion[]> {
  return request<IndexVersion[]>(`/documents/${id}/index-versions`)
}
