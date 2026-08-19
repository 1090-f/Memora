import CachedOutlined from '@mui/icons-material/CachedOutlined';
import CheckCircleRounded from '@mui/icons-material/CheckCircleRounded';
import ErrorOutlineRounded from '@mui/icons-material/ErrorOutlineRounded';
import RadioButtonUncheckedRounded from '@mui/icons-material/RadioButtonUncheckedRounded';
import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import {
  Alert,
  Box,
  Button,
  Chip,
  Divider,
  Stack,
  Typography,
} from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/api/queryKeys';
import { errorMessage } from '@/api/errors';
import { listDocumentIndexVersions, reindexDocument, retryDocumentProcessing } from '../api';
import { documentStatusLabel } from '../status';
import type { Document, DocumentProcessing } from '../types';

function formatBytes(value?: number) {
  if (value === undefined || value < 0) return null;
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function formatTime(value?: string) {
  if (!value) return null;
  return new Date(value).toLocaleString('zh-CN', { hour12: false });
}

const stages = [
  { key: 'parsing', label: '解析文件' },
  { key: 'cleaning', label: '清洗正文' },
  { key: 'chunking', label: '内容分块' },
  { key: 'embedding', label: '生成向量' },
  { key: 'keyword_indexing', label: '建立索引' },
] as const;

function stageIndex(status: string, failureStep?: string) {
  if (status === 'succeeded') return stages.length;
  if (status === 'failed') {
    const normalized = (failureStep || '').toLowerCase();
    const matched = stages.findIndex((stage) => normalized.includes(stage.key.split('_')[0]));
    return matched >= 0 ? matched : 0;
  }
  const matched = stages.findIndex((stage) => stage.key === status);
  return matched >= 0 ? matched : 0;
}

function DetailRows({ rows }: { rows: Array<{ label: string; value?: string | null; danger?: boolean }> }) {
  return (
    <Box>
      {rows.map((row) => (
        <Stack key={row.label} direction="row" spacing={2} alignItems="flex-start" sx={{ py: 1.15, borderBottom: '1px solid #edf0f4', '&:last-child': { borderBottom: 0 } }}>
          <Typography sx={{ width: 104, flexShrink: 0, color: '#7b879d', fontSize: 12.5 }}>{row.label}</Typography>
          <Typography sx={{ minWidth: 0, color: row.danger ? '#d14343' : '#27334c', fontSize: 13.5, fontWeight: 550, wordBreak: 'break-word' }}>{row.value ?? '—'}</Typography>
        </Stack>
      ))}
    </Box>
  );
}

// 技术信息面板：原文件信息、解析/分块/索引状态与版本统一放在这里，
// 作为「处理详情」Tab 内容在正文区域展示，不再使用侧边抽屉。
export function DocumentProcessingPanel({ document, processing }: {
  document: Document;
  processing?: DocumentProcessing;
}) {
  const queryClient = useQueryClient();
  const effectiveStatus = processing?.processing_status ?? document.processing_status;
  const failureStep = processing?.failure_step || document.failure_step;
  const activeIndexVersion = processing?.active_index_version ?? document.active_index_version;

  const versionsQuery = useQuery({
    queryKey: queryKeys.documentIndexVersions(document.id),
    queryFn: () => listDocumentIndexVersions(document.id),
  });
  const activeVersion = versionsQuery.data?.items.find((version) => version.version === activeIndexVersion);
  const allChunkCount = (versionsQuery.data?.items ?? []).reduce((sum, version) => sum + version.chunk_count, 0);
  const allVectorCount = (versionsQuery.data?.items ?? []).reduce((sum, version) => sum + version.vector_count, 0);

  const invalidateAll = (documentId: string) => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.documents(document.knowledge_base_id) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.document(documentId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.documentProcessing(documentId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.documentContent(documentId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.documentIndexVersions(documentId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(document.knowledge_base_id) });
  };

  const retryMutation = useMutation({
    mutationFn: retryDocumentProcessing,
    onSuccess: (_, documentId) => invalidateAll(documentId),
  });
  const reindexMutation = useMutation({
    mutationFn: reindexDocument,
    onSuccess: (_, documentId) => invalidateAll(documentId),
  });

  const fileRows: Array<{ label: string; value?: string | null; danger?: boolean }> = [
    { label: '原文件名称', value: document.original_file_name || document.title },
    { label: '格式', value: document.mime_type || (document.content_format === 'markdown' ? 'text/markdown' : 'text/plain') },
    { label: '最近处理时间', value: formatTime(document.updated_at) },
  ];
  const indexRows: Array<{ label: string; value?: string | null }> = [
    { label: '索引模式', value: document.index_mode === 'hybrid' ? '混合索引' : document.index_mode === 'keyword' ? '关键词索引' : '未建立索引' },
    { label: '内容版本', value: String(document.content_version) },
    { label: '分块版本', value: String(document.chunk_version) },
    { label: '当前索引版本', value: activeIndexVersion !== undefined ? `v${activeIndexVersion}` : '尚未建立' },
  ];
  const currentStage = stageIndex(effectiveStatus, failureStep);
  const chunkCount = activeVersion?.chunk_count ?? allChunkCount;
  const vectorCount = activeVersion?.vector_count ?? allVectorCount;

  return (
    <Stack spacing={3}>
      {retryMutation.error && <Alert severity="error">重新解析失败：{errorMessage(retryMutation.error)}</Alert>}
      {reindexMutation.error && <Alert severity="error">重新索引失败：{errorMessage(reindexMutation.error)}</Alert>}

      <Stack direction={{ xs: 'column', md: 'row' }} alignItems={{ md: 'center' }} spacing={1.5}>
        <Box sx={{ flexGrow: 1 }}>
          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
            <Typography component="h3" sx={{ color: '#17213b', fontSize: 18, fontWeight: 700 }}>处理概览</Typography>
            <Chip size="small" color={effectiveStatus === 'failed' ? 'error' : effectiveStatus === 'succeeded' ? 'success' : 'info'} label={documentStatusLabel(effectiveStatus, document.index_mode)} sx={{ height: 25, fontWeight: 650 }} />
          </Stack>
          <Typography sx={{ mt: 0.5, color: '#7a869b', fontSize: 13 }}>最近更新于 {formatTime(document.updated_at)}</Typography>
        </Box>
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Button size="small" variant="outlined" startIcon={<RefreshOutlined />} disabled={effectiveStatus !== 'failed' || retryMutation.isPending} onClick={() => retryMutation.mutate(document.id)}>
            {retryMutation.isPending ? '正在重新解析…' : '重新解析'}
          </Button>
          <Button size="small" variant="outlined" startIcon={<CachedOutlined />} disabled={effectiveStatus !== 'succeeded' || reindexMutation.isPending} onClick={() => reindexMutation.mutate(document.id)}>
            {reindexMutation.isPending ? '正在重新索引…' : '重新索引'}
          </Button>
        </Stack>
      </Stack>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'repeat(2, minmax(0, 1fr))', md: 'repeat(4, minmax(0, 1fr))' }, borderTop: '1px solid #e5e9f0', borderBottom: '1px solid #e5e9f0' }}>
        {[
          { label: '文件大小', value: formatBytes(document.file_size) ?? '—' },
          { label: '内容版本', value: `v${document.content_version}` },
          { label: '当前分块', value: String(chunkCount) },
          { label: '当前向量', value: String(vectorCount) },
        ].map((item, index) => (
          <Box key={item.label} sx={{ px: { xs: 1.5, md: 2.5 }, py: 2, borderLeft: { xs: index % 2 === 0 ? 0 : '1px solid #e5e9f0', md: index === 0 ? 0 : '1px solid #e5e9f0' }, borderTop: { xs: index >= 2 ? '1px solid #e5e9f0' : 0, md: 0 } }}>
            <Typography sx={{ color: '#7d899e', fontSize: 12 }}>{item.label}</Typography>
            <Typography sx={{ mt: 0.45, color: '#1e2a45', fontSize: 22, lineHeight: 1.2, fontWeight: 720 }}>{item.value}</Typography>
          </Box>
        ))}
      </Box>

      <Box>
        <Typography sx={{ mb: 1.5, color: '#35415a', fontSize: 13.5, fontWeight: 700 }}>处理流程</Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: `repeat(${stages.length}, minmax(0, 1fr))` }, gap: { xs: 1, sm: 0 } }}>
          {stages.map((stage, index) => {
            const failed = effectiveStatus === 'failed' && index === currentStage;
            const complete = effectiveStatus === 'succeeded' || index < currentStage;
            const active = !failed && !complete && index === currentStage;
            const color = failed ? '#d84d4d' : complete ? '#2d9b55' : active ? '#4f62e9' : '#a5adba';
            return (
              <Box key={stage.key} sx={{ position: 'relative', display: 'flex', alignItems: 'center', flexDirection: { xs: 'row', sm: 'column' }, gap: 0.8, textAlign: { sm: 'center' }, '&::after': index < stages.length - 1 ? { content: '""', position: 'absolute', display: { xs: 'none', sm: 'block' }, top: 12, left: 'calc(50% + 17px)', width: 'calc(100% - 34px)', height: 2, bgcolor: complete ? '#a8d9b7' : '#e1e5eb' } : undefined }}>
                <Box sx={{ position: 'relative', zIndex: 1, display: 'grid', placeItems: 'center', width: 26, height: 26, color, bgcolor: '#fff' }}>
                  {failed ? <ErrorOutlineRounded fontSize="small" /> : complete ? <CheckCircleRounded fontSize="small" /> : <RadioButtonUncheckedRounded fontSize="small" />}
                </Box>
                <Typography sx={{ color: active || failed || complete ? '#34405a' : '#929cad', fontSize: 12.5, fontWeight: active || failed ? 700 : 550 }}>{stage.label}</Typography>
              </Box>
            );
          })}
        </Box>
      </Box>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'minmax(0, 1fr) minmax(0, 1fr)' }, gap: { xs: 2.5, md: 5 } }}>
        <Box>
          <Typography sx={{ pb: 1, borderBottom: '1px solid #dfe4eb', color: '#35415a', fontSize: 13.5, fontWeight: 700 }}>文件与解析</Typography>
          <DetailRows rows={fileRows} />
        </Box>
        <Box>
          <Typography sx={{ pb: 1, borderBottom: '1px solid #dfe4eb', color: '#35415a', fontSize: 13.5, fontWeight: 700 }}>分块与索引</Typography>
          <DetailRows rows={indexRows} />
        </Box>
      </Box>

      {document.parse_warnings && document.parse_warnings.length > 0 && (
        <>
          <Divider />
          <Typography sx={{ color: '#35415a', fontSize: 13.5, fontWeight: 700 }}>解析警告</Typography>
          <Stack spacing={0.5}>
            {document.parse_warnings.map((warning) => (
              <Alert key={warning} severity="warning" sx={{ py: 0.5 }}>{warning}</Alert>
            ))}
          </Stack>
        </>
      )}

      {versionsQuery.data && versionsQuery.data.items.length > 0 && (
        <>
          <Divider />
          <Typography sx={{ color: '#35415a', fontSize: 13.5, fontWeight: 700 }}>索引版本</Typography>
          <Stack spacing={0} sx={{ borderTop: '1px solid #e5e9f0' }}>
            {versionsQuery.data.items.map((version) => (
              <Box key={version.version} sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', columnGap: 2, rowGap: 0.7, py: 1.2, borderBottom: '1px solid #e5e9f0' }}>
                <Stack direction="row" spacing={0.7} alignItems="center" sx={{ minWidth: 76 }}><Typography sx={{ fontSize: 13.5, fontWeight: 700 }}>v{version.version}</Typography>{version.version === activeIndexVersion && <Chip size="small" color="primary" label="当前" sx={{ height: 20, fontSize: 10.5 }} />}</Stack>
                <Typography sx={{ color: '#66738a', fontSize: 12.5, minWidth: 82 }}>{version.chunk_count} 个分块</Typography>
                <Typography sx={{ color: '#66738a', fontSize: 12.5, minWidth: 82 }}>{version.vector_count} 个向量</Typography>
                <Typography sx={{ color: '#66738a', fontSize: 12.5, ml: { sm: 'auto' } }}>{version.status}</Typography>
              </Box>
            ))}
          </Stack>
        </>
      )}
      {versionsQuery.error && <Alert severity="warning">索引版本加载失败：{errorMessage(versionsQuery.error)}</Alert>}

    </Stack>
  );
}
