import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import TuneOutlined from '@mui/icons-material/TuneOutlined';
import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { errorMessage } from '@/api/errors';
import { getDocumentTextPreview } from '../../api';
import type { DocumentPreview, PreviewFallback, PreviewStatus, PreviewType } from '../../types';
import { BlobViewer } from './BlobViewer';
import { MarkdownViewer } from './MarkdownViewer';
import { PdfViewer } from './PdfViewer';
import { TableViewer } from './TableViewer';
import { TextViewer } from './TextViewer';

interface PreviewMode {
  key: string;
  type: PreviewType;
  status: PreviewStatus;
  contentUrl?: string;
  primary: boolean;
}

const label: Record<PreviewType, string> = {
  pdf: '原始版式', image: '原图', table: '表格', markdown: '解析正文', text: '解析正文', download: '下载', none: '不可预览',
};

const PROCESSING_KEY = '__processing';

export function PreviewHost({ descriptor, title, onRetry, retrying, processingContent }: {
  descriptor: DocumentPreview;
  title: string;
  onRetry: () => void;
  retrying: boolean;
  processingContent: ReactNode;
}) {
  const modes = useMemo(() => {
    const result: PreviewMode[] = [{
      key: `primary-${descriptor.preview_type}`,
      type: descriptor.preview_type,
      status: descriptor.status,
      contentUrl: descriptor.content_url,
      primary: true,
    }];
    const seen = new Set([descriptor.preview_type]);
    descriptor.fallbacks.forEach((fallback: PreviewFallback) => {
      if (fallback.preview_type === 'download' || fallback.preview_type === 'none' || seen.has(fallback.preview_type)) return;
      seen.add(fallback.preview_type);
      result.push({ key: `fallback-${fallback.preview_type}`, type: fallback.preview_type, status: fallback.status, contentUrl: fallback.content_url, primary: false });
    });
    return result;
  }, [descriptor]);
  const preferred = modes[0]?.status === 'ready' ? modes[0] : modes.find((mode) => mode.status === 'ready') ?? modes[0];
  const [selectedKey, setSelectedKey] = useState(preferred?.key ?? '');
  useEffect(() => setSelectedKey(preferred?.key ?? ''), [descriptor.document_id, descriptor.content_version, descriptor.status, preferred?.key]);
  const selected = modes.find((mode) => mode.key === selectedKey) ?? preferred;
  const showProcessing = selectedKey === PROCESSING_KEY;

  return (
    <Stack spacing={2}>
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        flexWrap="wrap"
        sx={{
          position: 'sticky',
          top: 0,
          zIndex: 10,
          bgcolor: 'background.paper',
          py: 1,
          borderRadius: 1,
        }}
      >
        {modes.length > 1 && (
          <>
            {modes.map((mode) => (
              <Button key={mode.key} size="small" variant={selected?.key === mode.key && !showProcessing ? 'contained' : 'outlined'} onClick={() => setSelectedKey(mode.key)}>
                {label[mode.type]}{mode.status === 'processing' || mode.status === 'pending' ? '（生成中）' : mode.status === 'failed' ? '（失败）' : ''}
              </Button>
            ))}
            <Box aria-hidden sx={{ width: 1, height: 18, bgcolor: 'divider', mx: 0.25 }} />
          </>
        )}
        <Button size="small" variant={showProcessing ? 'contained' : 'outlined'} startIcon={<TuneOutlined />} onClick={() => setSelectedKey(PROCESSING_KEY)}>处理详情</Button>
      </Stack>
      {showProcessing ? processingContent : (selected && <ModeContent documentId={descriptor.document_id} mode={selected} title={title} error={selected.primary ? descriptor.error?.message : undefined} onRetry={onRetry} retrying={retrying} />)}
    </Stack>
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
    return <Alert severity="info">预览正在后台生成。可以先查看已就绪的解析正文，完成后页面会自动刷新。</Alert>;
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
    case 'image': return <BlobViewer documentId={documentId} type="image" contentUrl={mode.contentUrl} title={title} />;
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
