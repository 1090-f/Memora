import DownloadOutlined from '@mui/icons-material/DownloadOutlined';
import OpenInNewOutlined from '@mui/icons-material/OpenInNewOutlined';
import { Alert, Box, Button, Chip, Divider, Paper, Stack, Typography } from '@mui/material';
import { useInfiniteQuery, useMutation, useQuery } from '@tanstack/react-query';
import { useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { getDocumentPreview, getOriginalDocument, listDocumentIndexVersions, readDocumentContent } from '../api';
import { documentStatusLabel } from '../status';
import type { Document, DocumentProcessing } from '../types';

const sourceLabel = { manual: '手工文档', file: '文件导入', url: 'URL 导入' } as const;

const markdownSx = {
  overflowWrap: 'anywhere',
  '& > :first-of-type': { mt: 0 },
  '& > :last-child': { mb: 0 },
  '& h1, & h2, & h3, & h4, & h5, & h6': { mt: 2.5, mb: 1, lineHeight: 1.35 },
  '& p, & ul, & ol, & blockquote': { my: 1.25, lineHeight: 1.75 },
  '& pre': { overflowX: 'auto', p: 1.5, borderRadius: 1, bgcolor: 'action.hover' },
  '& code': { fontFamily: 'Consolas, "SFMono-Regular", monospace' },
  '& :not(pre) > code': { px: 0.5, py: 0.2, borderRadius: 0.5, bgcolor: 'action.hover' },
  '& table': { display: 'block', maxWidth: '100%', overflowX: 'auto', borderCollapse: 'collapse', my: 2 },
  '& th, & td': { border: 1, borderColor: 'divider', px: 1.25, py: 0.75, textAlign: 'left' },
  '& blockquote': { ml: 0, pl: 2, borderLeft: 4, borderColor: 'divider', color: 'text.secondary' },
  '& img': { maxWidth: '100%', maxHeight: 480, width: 'auto', height: 'auto', cursor: 'zoom-in' },
} as const;

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
  const shouldReadPreview = !document.content && effectiveStatus === 'succeeded';
  const previewQuery = useQuery({
    queryKey: [...queryKeys.documentContent(document.id), 'preview', document.content_version],
    queryFn: () => getDocumentPreview(document.id),
    enabled: shouldReadPreview,
    retry: false,
  });
  // 旧 Artifact 不可用时才回退到 Chunk 正文，保证已有数据仍可阅读。
  const shouldReadIndexedContent = shouldReadPreview && previewQuery.isError;
  const contentQuery = useInfiniteQuery({
    queryKey: [...queryKeys.documentContent(document.id), activeIndexVersion],
    queryFn: ({ pageParam }) => readDocumentContent(document.knowledge_base_id, document.id, {
      cursor: pageParam || undefined,
      max_tokens: 6000,
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
  const displayedContent = document.content || previewQuery.data?.content || indexedContent;
  const contentError = contentQuery.error;
  const fetchNextContentPage = contentQuery.fetchNextPage;
  const hasNextContentPage = contentQuery.hasNextPage;
  const isFetchingNextContentPage = contentQuery.isFetchingNextPage;
  const fileName = (document.original_file_name || document.title).toLowerCase();
  const mimeType = (document.mime_type || '').toLowerCase();
  const isPdf = mimeType === 'application/pdf' || fileName.endsWith('.pdf');
  const isDocx = mimeType === 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' || fileName.endsWith('.docx');
  const isOffice = isDocx || fileName.endsWith('.xlsx') || fileName.endsWith('.pptx');
  const isMarkdown = mimeType === 'text/markdown' || mimeType === 'text/x-markdown' || fileName.endsWith('.md') || fileName.endsWith('.markdown');
  const previewFormat = (previewQuery.data?.format || '').toLowerCase();
  const renderAsMarkdown = document.source_type !== 'url' && (isMarkdown || isPdf || isOffice || ['markdown', 'pdf', 'docx', 'xlsx', 'pptx'].includes(previewFormat));
  const canOpenOriginal = document.source_type === 'file' && (isPdf || isDocx);
  const originalMutation = useMutation({
    mutationFn: ({ inline }: { inline: boolean; previewWindow: Window | null }) => getOriginalDocument(document.id, inline),
    onSuccess: (blob, { inline, previewWindow }) => {
      const objectURL = URL.createObjectURL(blob);
      if (inline && previewWindow) {
        previewWindow.opener = null;
        previewWindow.location.href = objectURL;
      } else {
        previewWindow?.close();
        const anchor = window.document.createElement('a');
        anchor.href = objectURL;
        anchor.download = document.original_file_name || document.title;
        window.document.body.appendChild(anchor);
        anchor.click();
        anchor.remove();
      }
      window.setTimeout(() => URL.revokeObjectURL(objectURL), 60_000);
    },
    onError: (_error, { previewWindow }) => previewWindow?.close(),
  });

  const openOriginal = () => {
    const inline = isPdf;
    const previewWindow = inline ? window.open('about:blank', '_blank') : null;
    originalMutation.mutate({ inline, previewWindow });
  };

  // 文档预览面向人工阅读：保留后端的安全分页，但在前端自动连续读取，不暴露工具游标和分页操作。
  useEffect(() => {
    if (shouldReadIndexedContent && hasNextContentPage && !isFetchingNextContentPage && !contentError) {
      void fetchNextContentPage();
    }
  }, [contentError, fetchNextContentPage, hasNextContentPage, isFetchingNextContentPage, shouldReadIndexedContent]);

  return (
    <Paper variant="outlined" sx={{ p: 3, minHeight: 360, overflow: 'hidden' }}>
      <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>{document.title}</Typography>
        <Chip size="small" label={sourceLabel[document.source_type]} />
        <Chip
          size="small"
          color={effectiveStatus === 'failed' ? 'error' : effectiveStatus === 'succeeded' ? 'success' : 'info'}
          label={documentStatusLabel(effectiveStatus, document.index_mode)}
        />
        {canOpenOriginal && (
          <Button
            size="small"
            variant="outlined"
            startIcon={isPdf ? <OpenInNewOutlined /> : <DownloadOutlined />}
            disabled={originalMutation.isPending}
            onClick={openOriginal}
          >
            {originalMutation.isPending ? '正在读取…' : isPdf ? '查看原文件' : '下载原文件'}
          </Button>
        )}
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
      {document.parse_warnings && document.parse_warnings.length > 0 && (
        <Alert severity="warning" sx={{ mt: 2 }}>
          {document.parse_warnings.map((warning) => (
            <Box key={warning} component="div">{warning}</Box>
          ))}
        </Alert>
      )}
      {originalMutation.error && <Alert severity="warning" sx={{ mt: 2 }}>原文件读取失败：{errorMessage(originalMutation.error)}</Alert>}

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
      {displayedContent ? (
        <>
          {renderAsMarkdown ? (
            <Box sx={markdownSx}>
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  img: ({ node: _node, ...props }) => (
                    // 点击图片在新标签页打开原图（asset URL 已带签名）。
                    <img
                      {...props}
                      onClick={() => {
                        const src = props.src;
                        if (src && !src.startsWith('data:')) window.open(src, '_blank', 'noopener');
                      }}
                    />
                  ),
                }}
              >
                {displayedContent}
              </ReactMarkdown>
            </Box>
          ) : (
            <Typography component="pre" sx={{ m: 0, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontFamily: 'inherit' }}>
              {displayedContent}
            </Typography>
          )}
          {contentQuery.isFetchingNextPage && <Typography mt={2} color="text.secondary">正在加载剩余正文…</Typography>}
          {contentQuery.error && contentQuery.hasNextPage && (
            <Alert
              severity="warning"
              sx={{ mt: 2 }}
              action={<Button color="inherit" size="small" onClick={() => void contentQuery.fetchNextPage()}>重试</Button>}
            >
              剩余正文加载失败：{errorMessage(contentQuery.error)}
            </Alert>
          )}
        </>
      ) : previewQuery.isPending && shouldReadPreview ? (
        <Typography color="text.secondary">正在读取完整解析正文…</Typography>
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
