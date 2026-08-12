import DownloadOutlined from '@mui/icons-material/DownloadOutlined';
import OpenInNewOutlined from '@mui/icons-material/OpenInNewOutlined';
import TuneOutlined from '@mui/icons-material/TuneOutlined';
import { Alert, Button, Chip, Divider, Paper, Stack, Typography } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { getDocumentPreview, getOriginalDocument, retryDocumentPreview } from '../api';
import { documentStatusLabel } from '../status';
import type { Document, DocumentProcessing } from '../types';
import { DocumentProcessingDrawer } from './DocumentProcessingDrawer';
import { PreviewHost } from './preview/PreviewHost';

const sourceLabel = { manual: '手工文档', file: '文件导入', url: 'URL 导入' } as const;

function mainStatusLabel(status: DocumentProcessing['processing_status'], indexMode: Document['index_mode']) {
  if (status === 'succeeded') {
    if (indexMode === 'hybrid') return '已解析 · 已建立索引';
    if (indexMode === 'keyword') return '已解析 · 已建立关键词索引';
    return '已解析 · 未建立索引';
  }
  return documentStatusLabel(status, indexMode);
}

export function DocumentViewer({ document, processing }: { document: Document; processing?: DocumentProcessing }) {
  const queryClient = useQueryClient();
  const [processingOpen, setProcessingOpen] = useState(false);
  const failureReason = processing?.failure_reason || document.failure_reason;
  const failureStep = processing?.failure_step || document.failure_step;
  const effectiveStatus = processing?.processing_status ?? document.processing_status;
  const descriptorQuery = useQuery({
    queryKey: [...queryKeys.documentContent(document.id), 'preview-descriptor', document.content_version],
    queryFn: () => getDocumentPreview(document.id),
    retry: false,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === 'pending' || status === 'processing' ? query.state.data?.retry_after_ms ?? 2000 : false;
    },
  });
  const retryPreview = useMutation({
    mutationFn: () => retryDocumentPreview(document.id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: queryKeys.documentContent(document.id) }),
  });
  const originalMutation = useMutation({
    mutationFn: () => getOriginalDocument(document.id),
    onSuccess: (blob) => {
      const objectURL = URL.createObjectURL(blob);
      const anchor = window.document.createElement('a');
      anchor.href = objectURL;
      anchor.download = document.original_file_name || document.title;
      window.document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(objectURL), 60_000);
    },
  });

  return (
    <Paper variant="outlined" sx={{ p: 3, minHeight: 360, overflow: 'hidden' }}>
      <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>{document.title}</Typography>
        <Chip size="small" label={sourceLabel[document.source_type]} />
        <Chip size="small" color={effectiveStatus === 'failed' ? 'error' : effectiveStatus === 'succeeded' ? 'success' : 'info'} label={mainStatusLabel(effectiveStatus, document.index_mode)} />
        {document.source_type === 'file' && (
          <Button size="small" variant="outlined" startIcon={<DownloadOutlined />} disabled={originalMutation.isPending} onClick={() => originalMutation.mutate()}>
            {originalMutation.isPending ? '正在下载…' : '下载原文件'}
          </Button>
        )}
        {document.source_type === 'url' && document.source_url && (
          <Button size="small" variant="outlined" startIcon={<OpenInNewOutlined />} component="a" href={document.source_url} target="_blank" rel="noopener noreferrer">打开来源页面</Button>
        )}
        <Button size="small" variant="text" startIcon={<TuneOutlined />} onClick={() => setProcessingOpen(true)}>处理详情</Button>
      </Stack>
      {document.source_url && <Typography mt={1} variant="body2" color="text.secondary">来源：{document.source_url}</Typography>}
      {failureReason && <Alert severity="error" sx={{ mt: 2 }}>失败步骤：{failureStep || '未知'}；{failureReason}</Alert>}
      {originalMutation.error && <Alert severity="warning" sx={{ mt: 2 }}>原文件读取失败：{errorMessage(originalMutation.error)}</Alert>}
      {retryPreview.error && <Alert severity="warning" sx={{ mt: 2 }}>预览重试失败：{errorMessage(retryPreview.error)}</Alert>}
      <Divider sx={{ my: 2 }} />
      {descriptorQuery.isPending ? (
        <Typography color="text.secondary">正在获取预览方式…</Typography>
      ) : descriptorQuery.error ? (
        <Alert severity="warning">预览信息读取失败：{errorMessage(descriptorQuery.error)}</Alert>
      ) : descriptorQuery.data ? (
        <PreviewHost descriptor={descriptorQuery.data} title={document.title} onRetry={() => retryPreview.mutate()} retrying={retryPreview.isPending} />
      ) : null}
      <DocumentProcessingDrawer open={processingOpen} onClose={() => setProcessingOpen(false)} document={document} processing={processing} />
    </Paper>
  );
}
