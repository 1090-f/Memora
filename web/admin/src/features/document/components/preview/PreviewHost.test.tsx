import { describe, expect, it } from 'vitest';
import { buildTabs } from './previewTabs';

describe('buildTabs', () => {
  it('uses browser Office preview first and exposes an optional PDF fallback', () => {
    const tabs = buildTabs([
      { key: 'primary-docx', type: 'docx', status: 'ready', contentUrl: '/original' },
      { key: 'fallback-pdf', type: 'pdf', status: 'processing' },
      { key: 'fallback-markdown', type: 'markdown', status: 'ready', contentUrl: '/text' },
    ]);

    expect(tabs.map((tab) => [tab.key, tab.title, tab.mode?.type])).toEqual([
      ['source', '文件预览', 'docx'],
      ['pdf-fallback', 'PDF 预览', 'pdf'],
      ['parsed', '解析正文', 'markdown'],
    ]);
  });

  it('keeps the structured table as the only XLSX source preview', () => {
    const tabs = buildTabs([
      { key: 'primary-table', type: 'table', status: 'ready', contentUrl: '/table' },
      { key: 'fallback-pdf', type: 'pdf', status: 'ready', contentUrl: '/rendered' },
      { key: 'fallback-markdown', type: 'markdown', status: 'ready', contentUrl: '/text' },
    ]);

    expect(tabs.map((tab) => [tab.key, tab.title, tab.mode?.type])).toEqual([
      ['source', '原版预览', 'table'],
      ['parsed', '解析正文', 'markdown'],
    ]);
  });
});
