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
  children: DirectoryNode[];
}

export interface Document {
  id: string;
  knowledge_base_id: string;
  directory_id: string | null;
  title: string;
  content: string;
  source_type: DocumentSourceType;
  source_url: string | null;
  processing_status: DocumentProcessingStatus;
  content_version: number;
  active_index_version: number | null;
  created_at: string;
  updated_at: string;
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
