import DownloadOutlined from '@mui/icons-material/DownloadOutlined';
import InsertDriveFileOutlined from '@mui/icons-material/InsertDriveFileOutlined';
import OpenInNewOutlined from '@mui/icons-material/OpenInNewOutlined';
import { Alert, Box, Button, Chip, Divider, Paper, Stack, Typography } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { getDocumentPreview, getOriginalDocument, retryDocumentPreview } from '../api';
import { documentStatusLabel } from '../status';
import type { Document, DocumentProcessing } from '../types';
import { DocumentProcessingPanel } from './DocumentProcessingPanel';
import { PreviewHost } from './preview/PreviewHost';

const sourceLabel = { manual: '手工文档', file: '文件导入', url: 'URL 导入' } as const;

function fileSizeLabel(size?: number) {
  if (!size) return undefined;
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function fileExtension(name: string) {
  const extension = name.split('.').pop();
  return extension ? `${extension.toUpperCase()} 文件` : '文件';
}

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
  const failureReason = processing?.failure_reason || document.failure_reason;
  const failureStep = processing?.failure_step || document.failure_step;
  const effectiveStatus = processing?.processing_status ?? document.processing_status;
  const descriptorQuery = useQuery({
    queryKey: queryKeys.documentPreview(document.id),
    queryFn: () => getDocumentPreview(document.id),
    retry: false,
    refetchInterval: (query) => {
      const descriptor = query.state.data;
      const isGenerating = descriptor?.status === 'pending'
        || descriptor?.status === 'processing'
        || descriptor?.fallbacks.some((fallback) => fallback.status === 'pending' || fallback.status === 'processing');
      return isGenerating ? descriptor?.retry_after_ms ?? 2000 : false;
    },
  });
  const retryPreview = useMutation({
    mutationFn: () => retryDocumentPreview(document.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documentPreview(document.id) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.documentContent(document.id) });
    },
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
    <Paper elevation={0} sx={{ minHeight: 360, bgcolor: 'transparent' }}>
      <Stack direction={{ xs: 'column', md: 'row' }} alignItems={{ md: 'center' }} spacing={2.5} sx={{ px: { xs: 0, md: 0.25 }, py: 2.4 }}>
        <Box sx={{ width: 72, height: 72, display: 'grid', placeItems: 'center', borderRadius: 2.2, bgcolor: '#eaf8ee', color: '#35a860', flexShrink: 0 }}>
          <InsertDriveFileOutlined sx={{ fontSize: 48 }} />
        </Box>
        <Box sx={{ minWidth: 0, flexGrow: 1 }}>
          <Typography component="h2" sx={{ color: '#17213b', fontSize: { xs: 23, md: 28 }, fontWeight: 750, lineHeight: 1.25 }}>{document.title}</Typography>
          <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ mt: 1.35 }}>
            <Chip size="small" label={fileExtension(document.original_file_name || document.title)} sx={{ height: 30, bgcolor: '#f5f7fc', color: '#69758d', borderRadius: 1.3 }} />
            {fileSizeLabel(document.file_size) && <Chip size="small" label={fileSizeLabel(document.file_size)} sx={{ height: 30, bgcolor: '#f5f7fc', color: '#69758d', borderRadius: 1.3 }} />}
            <Chip size="small" label={`上传于 ${new Date(document.created_at).toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })}`} sx={{ height: 30, bgcolor: '#f5f7fc', color: '#69758d', borderRadius: 1.3 }} />
          </Stack>
        </Box>
        <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap" useFlexGap sx={{ flexShrink: 0 }}>
          <Chip size="small" label={sourceLabel[document.source_type]} sx={{ height: 32, bgcolor: '#f3f4f6', color: '#545a68' }} />
          <Chip size="small" color={effectiveStatus === 'failed' ? 'error' : effectiveStatus === 'succeeded' ? 'success' : 'info'} label={mainStatusLabel(effectiveStatus, document.index_mode)} sx={{ height: 32, fontWeight: 650 }} />
          {document.source_type === 'file' && (
            <Button variant="contained" startIcon={<DownloadOutlined />} disabled={originalMutation.isPending} onClick={() => originalMutation.mutate()} sx={{ height: 42, px: 2, borderRadius: 1.5, boxShadow: '0 6px 14px rgba(79, 83, 232, .22)' }}>
              {originalMutation.isPending ? '正在下载…' : '下载原文件'}
            </Button>
          )}
          {document.source_type === 'url' && document.source_url && (
            <Button variant="outlined" startIcon={<OpenInNewOutlined />} component="a" href={document.source_url} target="_blank" rel="noopener noreferrer">打开来源页面</Button>
          )}
        </Stack>
      </Stack>
      {document.source_url && <Typography mt={1} variant="body2" color="text.secondary">来源：{document.source_url}</Typography>}
      {failureReason && <Alert severity="error" sx={{ mt: 2 }}>失败步骤：{failureStep || '未知'}；{failureReason}</Alert>}
      {originalMutation.error && <Alert severity="warning" sx={{ mt: 2 }}>原文件读取失败：{errorMessage(originalMutation.error)}</Alert>}
      {retryPreview.error && <Alert severity="warning" sx={{ mt: 2 }}>预览重试失败：{errorMessage(retryPreview.error)}</Alert>}
      <Divider sx={{ mb: 2.3 }} />
      {descriptorQuery.isPending ? (
        <Typography color="text.secondary">正在获取预览方式…</Typography>
      ) : descriptorQuery.error ? (
        <Alert severity="warning">预览信息读取失败：{errorMessage(descriptorQuery.error)}</Alert>
      ) : descriptorQuery.data ? (
        <PreviewHost
          descriptor={descriptorQuery.data}
          title={document.title}
          onRetry={() => retryPreview.mutate()}
          retrying={retryPreview.isPending}
          processingContent={<DocumentProcessingPanel document={document} processing={processing} />}
        />
      ) : null}
    </Paper>
  );
}
