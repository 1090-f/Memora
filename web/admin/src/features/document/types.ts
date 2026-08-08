export type DocumentSourceType = 'manual' | 'file' | 'url';
export type DocumentProcessingStatus =
  | 'pending' | 'parsing' | 'cleaning' | 'chunking' | 'embedding'
  | 'keyword_indexing' | 'succeeded' | 'failed';
export type ImportTaskStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'skipped';

export interface DirectoryNode {
  id: string;
  name: string;
  parent_id: string | null;
  depth: number;
  sort_order: number;
  is_default: boolean;
  children: DirectoryNode[];
}

export interface DocumentListItem {
  id: string;
  title: string;
  directory_id: string | null;
  source_type: DocumentSourceType;
  processing_status: DocumentProcessingStatus;
  file_size: number | null;
  created_at: string;
  updated_at: string;
}

export interface Document {
  id: string;
  knowledge_base_id: string;
  directory_id: string | null;
  title: string;
  content: string;
  source_type: DocumentSourceType;
  source_url: string | null;
  original_file_name: string | null;
  file_size: number | null;
  mime_type: string | null;
  processing_status: DocumentProcessingStatus;
  failure_step: string | null;
  failure_reason: string | null;
  content_version: number;
  chunk_version: number;
  active_index_version: number | null;
  created_at: string;
  updated_at: string;
}

export interface DocumentProcessing {
  document_id: string;
  processing_status: DocumentProcessingStatus;
  current_index_version: number;
  active_index_version: number;
  failure_step: string | null;
  failure_reason: string | null;
}

export interface ImportTask {
  id: string;
  source_type: 'file' | 'url';
  file_name: string | null;
  file_size: number | null;
  mime_type: string | null;
  source_url: string | null;
  status: ImportTaskStatus;
  current_step: string | null;
  failure_reason: string | null;
  document_id: string | null;
  created_at: string;
  completed_at: string | null;
}

export interface CreateDirectoryInput {
  name: string;
  parent_id?: string;
  sort_order?: number;
}
