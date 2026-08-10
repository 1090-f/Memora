import { Alert, Box, Chip, Divider, Paper, Stack, Typography } from '@mui/material';
import type { Document, DocumentProcessing } from '../types';

const sourceLabel = { manual: '手工文档', file: '文件导入', url: 'URL 导入' } as const;

function formatBytes(value?: number) {
  if (value === undefined) return null;
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

export function DocumentViewer({ document, processing }: { document: Document; processing?: DocumentProcessing }) {
  // 处理状态接口的实时数据优先于文档详情中的快照。
  const failureReason = processing?.failure_reason || document.failure_reason;
  const failureStep = processing?.failure_step || document.failure_step;
  const activeIndexVersion = processing?.active_index_version ?? document.active_index_version;
  return (
    <Paper variant="outlined" sx={{ p: 3, minHeight: 360, overflow: 'hidden' }}>
      <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>{document.title}</Typography>
        <Chip size="small" label={sourceLabel[document.source_type]} />
        <Chip
          size="small"
          color={document.processing_status === 'failed' ? 'error' : document.processing_status === 'succeeded' ? 'success' : 'info'}
          label={document.processing_status}
        />
      </Stack>
      <Stack direction="row" spacing={2} mt={1} color="text.secondary" flexWrap="wrap">
        {document.original_file_name && <Typography variant="caption">文件：{document.original_file_name}</Typography>}
        {document.mime_type && <Typography variant="caption">类型：{document.mime_type}</Typography>}
        {document.file_size !== undefined && <Typography variant="caption">大小：{formatBytes(document.file_size)}</Typography>}
        <Typography variant="caption">内容版本：{document.content_version}</Typography>
        <Typography variant="caption">分块版本：{document.chunk_version}</Typography>
        <Typography variant="caption">活动索引：{activeIndexVersion || '尚未建立'}</Typography>
      </Stack>
      {document.source_url && <Typography mt={1} variant="body2" color="text.secondary">来源：{document.source_url}</Typography>}
      {failureReason && <Alert severity="error" sx={{ mt: 2 }}>失败步骤：{failureStep || '未知'}；{failureReason}</Alert>}
      <Divider sx={{ my: 2 }} />
      {document.content ? (
        <Typography component="pre" sx={{ m: 0, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontFamily: 'inherit' }}>
          {document.content}
        </Typography>
      ) : (
        <Box py={5} textAlign="center">
          {/* 任务包 08 接入正文读取前，文件文档只展示元数据与索引状态。 */}
          <Typography color="text.secondary">该文档没有可直接展示的正文。</Typography>
          <Typography variant="body2" color="text.secondary">文件解析结果已用于分块和索引；正文读取能力将在任务包 08 接入。</Typography>
        </Box>
      )}
    </Paper>
  );
}
