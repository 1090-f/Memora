import { describe, expect, it } from 'vitest';

import { documentStatusFilterParams, documentStatusLabel, processingLabel } from './status';

describe('document status mapping', () => {
  it('has a user-facing label for every processing state', () => {
    expect(processingLabel).toEqual({
      pending: '待处理',
      parsing: '解析中',
      cleaning: '清洗中',
      chunking: '分段中',
      embedding: '向量化中',
      keyword_indexing: '关键词索引中',
      succeeded: '已完成',
      failed: '失败',
    });
  });

  it('distinguishes searchable index modes from an unusable success state', () => {
    expect(documentStatusLabel('succeeded', 'hybrid')).toBe('已完成（混合索引）');
    expect(documentStatusLabel('succeeded', 'keyword')).toBe('已完成（仅关键词）');
    expect(documentStatusLabel('succeeded', 'none')).toBe('处理成功（不可检索）');
    expect(documentStatusLabel('embedding', 'none')).toBe('向量化中');
  });

  it('maps ready filters to backend processing and index parameters', () => {
    expect(documentStatusFilterParams('')).toEqual({});
    expect(documentStatusFilterParams('keyword_ready')).toEqual({ processing_status: 'succeeded', index_mode: 'keyword' });
    expect(documentStatusFilterParams('hybrid_ready')).toEqual({ processing_status: 'succeeded', index_mode: 'hybrid' });
    expect(documentStatusFilterParams('failed')).toEqual({ processing_status: 'failed' });
  });
});
