export type DocumentSourceType = 'manual' | 'file' | 'url';
export type DocumentProcessingStatus =
  | 'pending' | 'parsing' | 'cleaning' | 'chunking' | 'embedding'
  | 'keyword_indexing' | 'succeeded' | 'failed';
export type ImportTaskStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'skipped';

// 后端对空指针字段使用 omitempty，因此可选字段以 undefined 表达而不是强制 null。
export interface DirectoryNode {
  id: string;
  name: string;
  parent_id?: string;
  depth: number;
  sort_order: number;
  is_default: boolean;
  children: DirectoryNode[];
}

export interface DocumentListItem {
  id: string;
  title: string;
  directory_id?: string;
  source_type: DocumentSourceType;
  processing_status: DocumentProcessingStatus;
  file_size?: number;
  created_at: string;
  updated_at: string;
}

export interface Document {
  id: string;
  knowledge_base_id: string;
  directory_id?: string;
  title: string;
  content?: string;
  source_type: DocumentSourceType;
  source_url?: string;
  original_file_name?: string;
  file_size?: number;
  mime_type?: string;
  processing_status: DocumentProcessingStatus;
  failure_step?: string;
  failure_reason?: string;
  content_version: number;
  chunk_version: number;
  active_index_version?: number;
  created_at: string;
  updated_at: string;
}

export interface DocumentProcessing {
  document_id: string;
  processing_status: DocumentProcessingStatus;
  current_index_version: number;
  active_index_version: number;
  failure_step: string;
  failure_reason: string;
}

export interface DocumentIndexVersion {
  version: number;
  chunk_count: number;
  vector_count: number;
  status: string;
  created_at: string;
}

export interface DocumentReadPage {
  document_id: string;
  title: string;
  content: string;
  next_cursor?: string;
  truncated: boolean;
  citation: Citation;
}

export interface ImportTask {
  id: string;
  source_type: 'file' | 'url';
  file_name?: string;
  file_size?: number;
  mime_type?: string;
  source_url?: string;
  status: ImportTaskStatus;
  current_step?: string;
  failure_reason?: string;
  document_id?: string;
  created_at: string;
  completed_at?: string;
}

export interface CreateDirectoryInput {
  name: string;
  parent_id?: string;
  sort_order?: number;
}

export interface CreateManualDocumentInput {
  title: string;
  content?: string;
  directory_id?: string;
  source_type: 'manual';
  source_url?: string;
}

export interface ImportURLInput {
  url: string;
  directory_id?: string;
  duplicate_policy?: 'create_new' | 'skip';
}

export interface ImportSubmission {
  task_id: string;
  file_name: string;
  status: ImportTaskStatus;
}

export interface DocumentListParams {
  page?: number;
  page_size?: number;
  keyword?: string;
  directory_id?: string;
  processing_status?: DocumentProcessingStatus;
  source_type?: DocumentSourceType;
}
import type { Citation } from '@/features/rag/types';
