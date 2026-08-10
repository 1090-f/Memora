import type {
  DocumentIndexMode,
  DocumentListParams,
  DocumentProcessingStatus,
} from './types';

export const processingLabel: Record<DocumentProcessingStatus, string> = {
  pending: '待处理',
  parsing: '解析中',
  cleaning: '清洗中',
  chunking: '分段中',
  embedding: '向量化中',
  keyword_indexing: '关键词索引中',
  succeeded: '已完成',
  failed: '失败',
};

export function documentStatusLabel(status: DocumentProcessingStatus, indexMode: DocumentIndexMode) {
  if (status !== 'succeeded') return processingLabel[status];
  if (indexMode === 'hybrid') return '已完成（混合索引）';
  if (indexMode === 'keyword') return '已完成（仅关键词）';
  return '已完成（索引未建立）';
}

export type DocumentStatusFilter =
  | ''
  | Exclude<DocumentProcessingStatus, 'succeeded'>
  | 'keyword_ready'
  | 'hybrid_ready';

export const documentStatusOptions: Array<{ value: Exclude<DocumentStatusFilter, ''>; label: string }> = [
  { value: 'pending', label: processingLabel.pending },
  { value: 'parsing', label: processingLabel.parsing },
  { value: 'cleaning', label: processingLabel.cleaning },
  { value: 'chunking', label: processingLabel.chunking },
  { value: 'embedding', label: processingLabel.embedding },
  { value: 'keyword_indexing', label: processingLabel.keyword_indexing },
  { value: 'keyword_ready', label: '已完成（仅关键词）' },
  { value: 'hybrid_ready', label: '已完成（混合索引）' },
  { value: 'failed', label: processingLabel.failed },
];

export function documentStatusFilterParams(status: DocumentStatusFilter): Pick<DocumentListParams, 'processing_status' | 'index_mode'> {
  if (status === 'keyword_ready') return { processing_status: 'succeeded', index_mode: 'keyword' };
  if (status === 'hybrid_ready') return { processing_status: 'succeeded', index_mode: 'hybrid' };
  if (status === '') return {};
  return { processing_status: status };
}
