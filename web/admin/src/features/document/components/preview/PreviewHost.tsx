import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import ArticleOutlined from '@mui/icons-material/ArticleOutlined';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import TuneOutlined from '@mui/icons-material/TuneOutlined';
import { Alert, Box, Button, Tab, Tabs, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { getDocumentTextPreviewById } from '../../api';
import type { DocumentPreview, PreviewStatus } from '../../types';
import { ImageViewer } from './ImageViewer';
import { MarkdownViewer } from './MarkdownViewer';
import { OfficeViewer } from './OfficeViewer';
import { PdfViewer } from './PdfViewer';
import { PresentationViewer } from './PresentationViewer';
import { TableViewer } from './TableViewer';
import { TextViewer } from './TextViewer';
import { buildTabs, collectModes, type PreviewMode } from './previewTabs';

const PROCESSING_KEY = '__processing';

const statusSuffix = (status: PreviewStatus) =>
  status === 'pending' || status === 'processing' ? '（生成中）' : status === 'failed' ? '（失败）' : '';


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
          : (selected?.mode && (selected.kind === 'parsed' && selected.sourceImage
            ? <ParsedImageContent documentId={descriptor.document_id} imageMode={selected.sourceImage} title={title} error={descriptor.error?.message} onRetry={onRetry} retrying={retrying} />
            : <ModeContent documentId={descriptor.document_id} mode={selected.mode} title={title} error={descriptor.error?.message} onRetry={onRetry} retrying={retrying} />))}
      </Box>
    </Box>
  );
}

function ParsedImageContent({ documentId, imageMode, title, error, onRetry, retrying }: {
  documentId: string;
  imageMode: PreviewMode;
  title: string;
  error?: string;
  onRetry: () => void;
  retrying: boolean;
}) {
  return (
    <Box>
      <ModeContent documentId={documentId} mode={imageMode} title={title} error={error} onRetry={onRetry} retrying={retrying} />
      <Box sx={{ mt: 2.5, pt: 2.5, borderTop: '1px solid #e6e9ef' }}>
        <TextResourceViewer documentId={documentId} markdown emptyMessage="图片中未识别到可显示的文字。" />
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
    case 'docx':
    case 'pptx': return <OfficeViewer documentId={documentId} contentUrl={mode.contentUrl} type={mode.type} />;
    case 'image': return <ImageViewer documentId={documentId} contentUrl={mode.contentUrl} title={title} />;
    case 'table': return <TableViewer documentId={documentId} contentUrl={mode.contentUrl} />;
    case 'markdown': return <TextResourceViewer documentId={documentId} markdown />;
    case 'text': return <TextResourceViewer documentId={documentId} markdown={false} />;
    default: return <Box py={4} textAlign="center"><Typography color="text.secondary">该预览方式暂不支持。</Typography></Box>;
  }
}

function TextResourceViewer({ documentId, markdown, emptyMessage = '该文档暂时没有可显示的正文。' }: { documentId: string; markdown: boolean; emptyMessage?: string }) {
  const query = useQuery({
    queryKey: queryKeys.documentPreviewText(documentId),
    queryFn: () => getDocumentTextPreviewById(documentId),
    retry: false,
    staleTime: 5 * 60_000,
  });
  if (query.isPending) return <Typography color="text.secondary">正在读取完整解析正文…</Typography>;
  if (query.error) return <Alert severity="warning">正文读取失败：{errorMessage(query.error)}</Alert>;
  if (query.data?.format === 'pptx' && query.data.slides?.length) return <PresentationViewer slides={query.data.slides} />;
  if (!query.data?.content) return <Typography color="text.secondary">{emptyMessage}</Typography>;
  return markdown || query.data.format === 'markdown'
    ? <MarkdownViewer content={query.data.content} />
    : <TextViewer content={query.data.content} />;
}
