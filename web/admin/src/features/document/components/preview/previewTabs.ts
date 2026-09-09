import type { DocumentPreview, PreviewFallback, PreviewStatus, PreviewType } from '../../types';

export interface PreviewMode {
  key: string;
  type: PreviewType;
  status: PreviewStatus;
  contentUrl?: string;
}

export interface PreviewTab {
  key: string;
  kind: 'source' | 'fallback' | 'parsed';
  title: string;
  mode?: PreviewMode;
  sourceImage?: PreviewMode;
}

// 汇总 primary 与回退模式，跳过 download/none 与重复类型。
export function collectModes(descriptor: DocumentPreview): PreviewMode[] {
  const result: PreviewMode[] = [{
    key: `primary-${descriptor.preview_type}`,
    type: descriptor.preview_type,
    status: descriptor.status,
    contentUrl: descriptor.content_url,
  }];
  const seen = new Set([descriptor.preview_type]);
  descriptor.fallbacks.forEach((fallback: PreviewFallback) => {
    if (fallback.preview_type === 'download' || fallback.preview_type === 'none' || seen.has(fallback.preview_type)) return;
    seen.add(fallback.preview_type);
    result.push({ key: `fallback-${fallback.preview_type}`, type: fallback.preview_type, status: fallback.status, contentUrl: fallback.content_url });
  });
  return result;
}

// DOCX/PPTX 直接浏览原文件，LibreOffice PDF 仅在启用时作为独立回退。
// XLSX 继续优先使用结构化 Table；解析正文始终保持独立入口。
export function buildTabs(modes: PreviewMode[]): PreviewTab[] {
  const byType = (type: PreviewType) => modes.find((mode) => mode.type === type);
  const table = byType('table');
  const office = byType('docx') ?? byType('pptx');
  const pdf = byType('pdf');
  const image = byType('image');
  const text = byType('markdown') ?? byType('text');

  const tabs: PreviewTab[] = [];
  const source = table ?? office ?? pdf ?? image ?? text;
  if (source) {
    const sourceTitle = source.type === 'table' ? '原版预览' : office === source ? '文件预览' : '原文预览';
    tabs.push({ key: 'source', kind: 'source', title: sourceTitle, mode: source });
  }
  if (office && pdf) {
    tabs.push({ key: 'pdf-fallback', kind: 'fallback', title: 'PDF 预览', mode: pdf });
  }
  if (text) {
    tabs.push({ key: 'parsed', kind: 'parsed', title: '解析正文', mode: text, sourceImage: image });
  }
  return tabs;
}
