import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import ArticleOutlined from '@mui/icons-material/ArticleOutlined';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import TuneOutlined from '@mui/icons-material/TuneOutlined';
import { Alert, Box, Button, Tab, Tabs, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { errorMessage } from '@/api/errors';
import { getDocumentTextPreview } from '../../api';
import type { DocumentPreview, PreviewFallback, PreviewStatus, PreviewType } from '../../types';
import { ImageViewer } from './ImageViewer';
import { MarkdownViewer } from './MarkdownViewer';
import { PdfViewer } from './PdfViewer';
import { TableViewer } from './TableViewer';
import { TextViewer } from './TextViewer';

interface PreviewMode {
  key: string;
  type: PreviewType;
  status: PreviewStatus;
  contentUrl?: string;
}

type TabKind = 'source' | 'table' | 'parsed';

interface PreviewTab {
  key: string;
  kind: TabKind;
  title: string;
  mode?: PreviewMode;
}

const PROCESSING_KEY = '__processing';

const statusSuffix = (status: PreviewStatus) =>
  status === 'pending' || status === 'processing' ? '（生成中）' : status === 'failed' ? '（失败）' : '';

// collectModes 汇总 primary 与回退模式，跳过 download/none 与重复类型。
function collectModes(descriptor: DocumentPreview): PreviewMode[] {
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

// buildTabs 统一所有文档类型的 Tab 结构：原版预览/原文预览 → 解析正文 → 处理详情（固定追加）。
//   xlsx 优先用「原版预览」（直接从原文件读取单元格渲染，数据层最忠实），
//   其余类型用「原文预览」（PDF 原文经 LibreOffice 转印 / 原图，txt/md 原文即正文）。
//   解析正文：一律展示解析文本；txt/md 原文与解析正文相同，仍作为独立 tab 保持结构统一。
function buildTabs(modes: PreviewMode[]): PreviewTab[] {
  const byType = (type: PreviewType) => modes.find((mode) => mode.type === type);
  const table = byType('table');
  const pdf = byType('pdf');
  const image = byType('image');
  const text = byType('markdown') ?? byType('text');

  const tabs: PreviewTab[] = [];
  const source = table ?? pdf ?? image ?? text;
  if (source) {
    tabs.push({ key: 'source', kind: 'source', title: source.type === 'table' ? '原版预览' : '原文预览', mode: source });
  }
  if (text) {
    tabs.push({ key: 'parsed', kind: 'parsed', title: '解析正文', mode: text });
  }
  return tabs;
}

export function PreviewHost({ descriptor, title, onRetry, retrying, processingContent }: {
  descriptor: DocumentPreview;
  title: string;
  onRetry: () => void;
  retrying: boolean;
  processingContent: ReactNode;
}) {
  const tabs = useMemo(() => buildTabs(collectModes(descriptor)), [descriptor]);
  const preferred = tabs.find((tab) => tab.mode?.status === 'ready') ?? tabs[0];
  const [selectedKey, setSelectedKey] = useState(preferred?.key ?? '');
  useEffect(() => setSelectedKey(preferred?.key ?? ''), [descriptor.document_id, descriptor.content_version, preferred?.key]);
  const selected = tabs.find((tab) => tab.key === selectedKey) ?? preferred;
  const showProcessing = selectedKey === PROCESSING_KEY;

  return (
    <Box>
      <Tabs
        value={showProcessing ? PROCESSING_KEY : selected?.key ?? false}
        onChange={(_, value: string) => setSelectedKey(value)}
        variant="scrollable"
        scrollButtons="auto"
        sx={{
          minHeight: 48,
          borderBottom: '1px solid #e6e9ef',
          '& .MuiTab-root': { minHeight: 48, px: 2, color: '#65708a', fontWeight: 650 },
          '& .Mui-selected': { color: '#4058e9' },
          '& .MuiTabs-indicator': { height: 3, borderRadius: '3px 3px 0 0' },
        }}
      >
        {tabs.map((tab) => (
          <Tab
            key={tab.key}
            value={tab.key}
            icon={tab.kind === 'parsed' ? <ArticleOutlined /> : <DescriptionOutlined />}
            iconPosition="start"
            label={`${tab.title}${tab.mode ? statusSuffix(tab.mode.status) : ''}`}
          />
        ))}
        {tabs.length > 0 && <Tab value={PROCESSING_KEY} icon={<TuneOutlined />} iconPosition="start" label="处理详情" />}
      </Tabs>
      <Box sx={{ pt: 2.5, minHeight: 360 }}>
        {showProcessing
          ? processingContent
          : (selected?.mode && <ModeContent documentId={descriptor.document_id} mode={selected.mode} title={title} error={descriptor.error?.message} onRetry={onRetry} retrying={retrying} />)}
      </Box>
    </Box>
  );
}

function ModeContent({ documentId, mode, title, error, onRetry, retrying }: {
  documentId: string;
  mode: PreviewMode;
  title: string;
  error?: string;
  onRetry: () => void;
  retrying: boolean;
}) {
  if (mode.status === 'pending' || mode.status === 'processing') {
    return <Alert severity="info">预览正在后台生成。可以先查看其他已就绪的视图，完成后页面会自动刷新。</Alert>;
  }
  if (mode.status === 'failed' || mode.status === 'unsupported') {
    return (
      <Alert severity="warning" action={mode.status === 'failed' ? <Button color="inherit" size="small" startIcon={<RefreshOutlined />} disabled={retrying} onClick={onRetry}>重试</Button> : undefined}>
        {error || (mode.status === 'unsupported' ? '该预览方式不可用。' : '预览生成失败，请使用其他阅读方式。')}
      </Alert>
    );
  }
  if (!mode.contentUrl) return <Typography color="text.secondary">没有可读取的预览资源。</Typography>;
  switch (mode.type) {
    case 'pdf': return <PdfViewer documentId={documentId} contentUrl={mode.contentUrl} title={title} />;
    case 'image': return <ImageViewer documentId={documentId} contentUrl={mode.contentUrl} title={title} />;
    case 'table': return <TableViewer documentId={documentId} contentUrl={mode.contentUrl} />;
    case 'markdown': return <TextResourceViewer documentId={documentId} contentUrl={mode.contentUrl} markdown />;
    case 'text': return <TextResourceViewer documentId={documentId} contentUrl={mode.contentUrl} markdown={false} />;
    default: return <Box py={4} textAlign="center"><Typography color="text.secondary">该预览方式暂不支持。</Typography></Box>;
  }
}

function TextResourceViewer({ documentId, contentUrl, markdown }: { documentId: string; contentUrl: string; markdown: boolean }) {
  const query = useQuery({
    queryKey: ['documents', documentId, 'preview-text', contentUrl],
    queryFn: () => getDocumentTextPreview(contentUrl),
    retry: false,
  });
  if (query.isPending) return <Typography color="text.secondary">正在读取完整解析正文…</Typography>;
  if (query.error) return <Alert severity="warning">正文读取失败：{errorMessage(query.error)}</Alert>;
  if (!query.data?.content) return <Typography color="text.secondary">该文档暂时没有可显示的正文。</Typography>;
  return markdown || query.data.format === 'markdown'
    ? <MarkdownViewer content={query.data.content} />
    : <TextViewer content={query.data.content} />;
}
