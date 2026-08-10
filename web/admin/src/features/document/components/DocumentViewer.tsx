import { Alert, Box, Button, Chip, Divider, Paper, Stack, Typography } from '@mui/material';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { listDocumentIndexVersions, readDocumentContent } from '../api';
import type { Document, DocumentProcessing } from '../types';

const sourceLabel = { manual: '手工文档', file: '文件导入', url: 'URL 导入' } as const;

function formatBytes(value?: number) {
  if (value === undefined) return null;
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

export function DocumentViewer({ document, processing }: { document: Document; processing?: DocumentProcessing }) {
  const failureReason = processing?.failure_reason || document.failure_reason;
  const failureStep = processing?.failure_step || document.failure_step;
  const activeIndexVersion = processing?.active_index_version ?? document.active_index_version;
  const effectiveStatus = processing?.processing_status ?? document.processing_status;
  const shouldReadIndexedContent = !document.content && effectiveStatus === 'succeeded';
  const contentQuery = useInfiniteQuery({
    queryKey: [...queryKeys.documentContent(document.id), activeIndexVersion],
    queryFn: ({ pageParam }) => readDocumentContent(document.knowledge_base_id, document.id, {
      cursor: pageParam || undefined,
      max_tokens: 2000,
    }),
    initialPageParam: '',
    getNextPageParam: (lastPage) => lastPage.truncated && lastPage.next_cursor ? lastPage.next_cursor : undefined,
    enabled: shouldReadIndexedContent,
  });
  const versionsQuery = useQuery({
    queryKey: [...queryKeys.documentIndexVersions(document.id), activeIndexVersion],
    queryFn: () => listDocumentIndexVersions(document.id),
    enabled: document.processing_status === 'succeeded',
  });
  const indexedContent = contentQuery.data?.pages.map((page) => page.content).join('') || '';

  return (
    <Paper variant="outlined" sx={{ p: 3, minHeight: 360, overflow: 'hidden' }}>
      <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>{document.title}</Typography>
        <Chip size="small" label={sourceLabel[document.source_type]} />
        <Chip size="small" color={document.processing_status === 'failed' ? 'error' : document.processing_status === 'succeeded' ? 'success' : 'info'} label={document.processing_status} />
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

      {versionsQuery.data?.items && versionsQuery.data.items.length > 0 && (
        <Stack direction="row" spacing={1} mt={2} alignItems="center" flexWrap="wrap" useFlexGap>
          <Typography variant="body2" fontWeight={700}>索引版本</Typography>
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
      )}
      {versionsQuery.error && <Alert severity="warning" sx={{ mt: 2 }}>索引版本加载失败：{errorMessage(versionsQuery.error)}</Alert>}

      <Divider sx={{ my: 2 }} />
      {document.content || indexedContent ? (
        <>
          <Typography component="pre" sx={{ m: 0, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontFamily: 'inherit' }}>
            {document.content || indexedContent}
          </Typography>
          {contentQuery.data?.pages.map((page, index) => (
            <Alert key={`${page.citation.chunk_id}-${index}`} severity="info" variant="outlined" sx={{ mt: 2 }}>
              引用 {index + 1}：{page.citation.document_title || page.title}
              {page.citation.source_location ? ` · ${JSON.stringify(page.citation.source_location)}` : ''}
            </Alert>
          ))}
          {contentQuery.hasNextPage && (
            <Button sx={{ mt: 2 }} variant="outlined" disabled={contentQuery.isFetchingNextPage} onClick={() => void contentQuery.fetchNextPage()}>
              {contentQuery.isFetchingNextPage ? '正在继续读取…' : '继续读取正文'}
            </Button>
          )}
        </>
      ) : contentQuery.isPending && shouldReadIndexedContent ? (
        <Typography color="text.secondary">正在读取已索引正文…</Typography>
      ) : contentQuery.error ? (
        <Alert severity="warning">正文读取失败：{errorMessage(contentQuery.error)}</Alert>
      ) : (
        <Box py={5} textAlign="center"><Typography color="text.secondary">该文档暂时没有可显示的正文。</Typography></Box>
      )}
    </Paper>
  );
}
