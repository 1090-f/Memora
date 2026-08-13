export type DocumentSourceType = 'manual' | 'file' | 'url';
export type DocumentProcessingStatus =
  | 'pending' | 'parsing' | 'cleaning' | 'chunking' | 'embedding'
  | 'keyword_indexing' | 'succeeded' | 'failed';
export type DocumentIndexMode = 'none' | 'keyword' | 'hybrid';
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
  index_mode: DocumentIndexMode;
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
  content_format?: 'txt' | 'markdown';
  source_type: DocumentSourceType;
  source_url?: string;
  original_file_name?: string;
  file_size?: number;
  mime_type?: string;
  processing_status: DocumentProcessingStatus;
  index_mode: DocumentIndexMode;
  failure_step?: string;
  failure_reason?: string;
  parse_warnings?: string[];
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

export type PreviewType = 'text' | 'markdown' | 'pdf' | 'image' | 'table' | 'download' | 'none';
export type PreviewStatus = 'pending' | 'processing' | 'ready' | 'failed' | 'unsupported';

export interface PreviewFallback {
  preview_type: PreviewType;
  status: PreviewStatus;
  content_url?: string;
  media_type?: string;
}

export interface PreviewError {
  code: string;
  message: string;
}

export interface DocumentPreview {
  document_id: string;
  content_version: number;
  preview_type: PreviewType;
  status: PreviewStatus;
  content_url?: string;
  media_type?: string;
  original_url?: string;
  retry_after_ms?: number;
  fallbacks: PreviewFallback[];
  error?: PreviewError;
}

export interface DocumentTextPreview {
  content: string;
  format: 'markdown' | 'txt' | string;
}

export interface PreviewSheetSummary {
  index: number;
  name: string;
  row_count: number;
  column_count: number;
}

export interface PreviewTableCell { column: number; value: string }
export interface PreviewTableRow { row: number; cells: PreviewTableCell[] }
export interface PreviewMergedCell {
  start_row: number;
  start_column: number;
  row_span: number;
  column_span: number;
}

export interface DocumentTablePreview {
  document_id: string;
  content_version: number;
  sheets: PreviewSheetSummary[];
  active_sheet: number;
  row_offset: number;
  row_limit: number;
  rows: PreviewTableRow[];
  merged_cells: PreviewMergedCell[];
  next_row_offset?: number;
}

export type ImageRefStatus = 'inline' | 'network' | 'matched' | 'pending';

export interface ImageScanItem {
  alt: string;
  ref: string;
  status: ImageRefStatus;
}

export interface ImageScanResult {
  refs: ImageScanItem[];
}

export interface ImportTask {
  id: string;
  batch_id?: string;
  source_path?: string;
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
  format?: 'txt' | 'markdown';
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
  batch_id?: string;
  source_path?: string;
  requires_confirmation?: boolean;
}

export interface ImportUploadRejected {
  source_path: string;
  code: string;
  message: string;
}

export interface ImportUploadResponse {
  batch_id: string;
  summary: { total: number; accepted: number; rejected: number };
  tasks: ImportSubmission[];
  rejected: ImportUploadRejected[];
}

export interface DocumentListParams {
  page?: number;
  page_size?: number;
  keyword?: string;
  directory_id?: string;
  processing_status?: DocumentProcessingStatus;
  index_mode?: DocumentIndexMode;
  source_type?: DocumentSourceType;
}
import type { Citation } from '@/features/rag/types';
