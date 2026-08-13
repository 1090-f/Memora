import CachedOutlined from '@mui/icons-material/CachedOutlined';
import CloseOutlined from '@mui/icons-material/CloseOutlined';
import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import {
  Alert,
  Box,
  Button,
  Chip,
  Divider,
  Drawer,
  IconButton,
  List,
  ListItem,
  ListItemText,
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
  return new Date(value).toLocaleString();
}

// 技术信息面板：原文件信息、解析/分块/索引状态与版本统一放在这里，
// 不在正文展示 Chunk 边界或 token 信息。
export function DocumentProcessingDrawer({ open, onClose, document, processing }: {
  open: boolean;
  onClose: () => void;
  document: Document;
  processing?: DocumentProcessing;
}) {
  const queryClient = useQueryClient();
  const effectiveStatus = processing?.processing_status ?? document.processing_status;
  const failureReason = processing?.failure_reason || document.failure_reason;
  const failureStep = processing?.failure_step || document.failure_step;
  const activeIndexVersion = processing?.active_index_version ?? document.active_index_version;

  const versionsQuery = useQuery({
    queryKey: queryKeys.documentIndexVersions(document.id),
    queryFn: () => listDocumentIndexVersions(document.id),
    enabled: open,
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

  const rows: Array<{ label: string; value?: string | null; danger?: boolean }> = [
    { label: '原文件名称', value: document.original_file_name || document.title },
    { label: '格式', value: document.mime_type || (document.content_format === 'markdown' ? 'text/markdown' : 'text/plain') },
    { label: '文件大小', value: formatBytes(document.file_size) },
    { label: '处理状态', value: documentStatusLabel(effectiveStatus, document.index_mode), danger: effectiveStatus === 'failed' },
    { label: '解析状态', value: documentStatusLabel(effectiveStatus, document.index_mode) },
    { label: '失败原因', value: failureReason ? `${failureStep ? `步骤 ${failureStep}；` : ''}${failureReason}` : null, danger: Boolean(failureReason) },
    { label: '内容版本', value: String(document.content_version) },
    { label: '分块版本', value: String(document.chunk_version) },
    { label: '当前索引版本', value: activeIndexVersion !== undefined ? `v${activeIndexVersion}` : '尚未建立' },
    { label: '分块数量', value: activeVersion ? String(activeVersion.chunk_count) : allChunkCount > 0 ? String(allChunkCount) : '0' },
    { label: '向量数量', value: activeVersion ? String(activeVersion.vector_count) : allVectorCount > 0 ? String(allVectorCount) : '0' },
    { label: '最近处理时间', value: formatTime(document.updated_at) },
  ];

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Stack spacing={2} sx={{ width: 440, p: 3 }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Typography variant="h6" sx={{ flexGrow: 1 }}>处理详情</Typography>
          <IconButton size="small" onClick={onClose}><CloseOutlined fontSize="small" /></IconButton>
        </Stack>

        {retryMutation.error && <Alert severity="error">重新解析失败：{errorMessage(retryMutation.error)}</Alert>}
        {reindexMutation.error && <Alert severity="error">重新索引失败：{errorMessage(reindexMutation.error)}</Alert>}

        <List disablePadding dense>
          {rows.map((row) => (
            <ListItem key={row.label} disableGutters sx={{ py: 0.5 }}>
              <ListItemText
                primary={row.label}
                primaryTypographyProps={{ variant: 'caption', color: 'text.secondary' }}
                secondary={row.value ?? '—'}
                secondaryTypographyProps={{
                  variant: 'body2',
                  color: row.danger ? 'error' : 'text.primary',
                  sx: { wordBreak: 'break-all' },
                }}
              />
            </ListItem>
          ))}
        </List>

        {document.parse_warnings && document.parse_warnings.length > 0 && (
          <>
            <Divider />
            <Typography variant="subtitle2" color="text.secondary">解析警告</Typography>
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
            <Typography variant="subtitle2" color="text.secondary">索引版本</Typography>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              {versionsQuery.data.items.map((version) => (
                <Chip
                  key={version.version}
                  size="small"
                  color={version.version === activeIndexVersion ? 'primary' : 'default'}
                  variant={version.version === activeIndexVersion ? 'filled' : 'outlined'}
                  label={`v${version.version} · ${version.chunk_count} chunks · ${version.vector_count} vectors · ${version.status}`}
                />
              ))}
            </Stack>
          </>
        )}
        {versionsQuery.error && <Alert severity="warning">索引版本加载失败：{errorMessage(versionsQuery.error)}</Alert>}

        <Divider />
        <Box>
          <Typography variant="subtitle2" color="text.secondary" mb={1}>操作</Typography>
          <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
            <Button
              size="small"
              variant="outlined"
              startIcon={<RefreshOutlined />}
              disabled={effectiveStatus !== 'failed' || retryMutation.isPending}
              onClick={() => retryMutation.mutate(document.id)}
            >
              {retryMutation.isPending ? '正在重新解析…' : '重新解析'}
            </Button>
            <Button
              size="small"
              variant="outlined"
              startIcon={<CachedOutlined />}
              disabled={effectiveStatus !== 'succeeded' || reindexMutation.isPending}
              onClick={() => reindexMutation.mutate(document.id)}
            >
              {reindexMutation.isPending ? '正在重新索引…' : '重新索引'}
            </Button>
          </Stack>
        </Box>
      </Stack>
    </Drawer>
  );
}
