import DownloadOutlined from '@mui/icons-material/DownloadOutlined';
import OpenInNewOutlined from '@mui/icons-material/OpenInNewOutlined';
import TuneOutlined from '@mui/icons-material/TuneOutlined';
import { Alert, Box, Button, Chip, Divider, Paper, Stack, Typography } from '@mui/material';
import { useInfiniteQuery, useMutation, useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { getDocumentPreview, getOriginalDocument, getRenderedDocument, readDocumentContent } from '../api';
import { documentStatusLabel } from '../status';
import type { Document, DocumentProcessing } from '../types';
import { DocumentProcessingDrawer } from './DocumentProcessingDrawer';

const sourceLabel = { manual: '手工文档', file: '文件导入', url: 'URL 导入' } as const;

// 展示模式：layout = 版式/原图（PDF/Office/图片），content = 解析正文。
type DocumentDisplayMode = 'layout' | 'content';

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

function mainStatusLabel(status: DocumentProcessing['processing_status'], indexMode: Document['index_mode']) {
  if (status === 'succeeded') {
    if (indexMode === 'hybrid') return '已解析 · 已建立索引';
    if (indexMode === 'keyword') return '已解析 · 已建立关键词索引';
    return '已解析 · 未建立索引';
  }
  return documentStatusLabel(status, indexMode);
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
  const isXlsx = mimeType === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' || fileName.endsWith('.xlsx');
  const isPptx = mimeType === 'application/vnd.openxmlformats-officedocument.presentationml.presentation' || fileName.endsWith('.pptx');
  const isOffice = isDocx || isXlsx || isPptx;
  const isImage = mimeType.startsWith('image/') ||
    ['.jpg', '.jpeg', '.png', '.bmp', '.tiff', '.tif', '.gif', '.webp'].some((ext) => fileName.endsWith(ext));
  const isMarkdown = mimeType === 'text/markdown' || mimeType === 'text/x-markdown' || fileName.endsWith('.md') || fileName.endsWith('.markdown');
  const isText = mimeType === 'text/plain' || fileName.endsWith('.txt');
  // 解析正文渲染规则：Markdown/PDF/Office/图片 的解析产物按 Markdown 渲染，
  // TXT 与 URL 抽取的普通文本按纯文本展示；不再依赖“图片被误标为 docx”触发。
  const renderAsMarkdown = !isText && document.source_type !== 'url' && (isMarkdown || isPdf || isOffice || isImage || document.content_format === 'markdown');
  const canDownloadOriginal = document.source_type === 'file';
  const hasFilePreview = canDownloadOriginal && (isPdf || isOffice || isImage);
  // 默认展示模式：PDF/Office/图片 → 版式/原图；Markdown/TXT/URL/手工文档 → 正文。
  const defaultMode: DocumentDisplayMode = hasFilePreview ? 'layout' : 'content';
  const [previewMode, setPreviewMode] = useState<DocumentDisplayMode>(defaultMode);
  const [processingOpen, setProcessingOpen] = useState(false);

  // 切换文档时恢复该格式的默认模式。
  useEffect(() => {
    setPreviewMode(defaultMode);
  }, [document.id, defaultMode]);

  // PDF 直接读取 MinIO 原文件；Office 文档经 LibreOffice 转 PDF；图片返回原图。
  // blob URL 解决 iframe 无法携带 Bearer header 的问题。
  const renderedQuery = useQuery({
    queryKey: [...queryKeys.documentContent(document.id), 'rendered', document.content_version],
    queryFn: () => getRenderedDocument(document.id),
    enabled: hasFilePreview && previewMode === 'layout' && !isImage,
    retry: false,
    staleTime: Infinity,
  });
  // 图片原图预览：直接读取原始文件（不走 LibreOffice 渲染）。
  const imageQuery = useQuery({
    queryKey: [...queryKeys.documentContent(document.id), 'image', document.content_version],
    queryFn: () => getOriginalDocument(document.id, true),
    enabled: hasFilePreview && previewMode === 'layout' && isImage,
    retry: false,
    staleTime: Infinity,
  });
  const filePreviewData = isImage ? imageQuery.data : renderedQuery.data;
  // 注意：StrictMode 下 useEffect cleanup 会立即 revoke，导致 iframe 偶发加载已回收的
  // blob URL（"未能加载 PDF"）。改为 useMemo 创建 + 延迟 60s 回收（iframe 早已加载完成），
  // 彻底绕开该竞态；旧 URL 在组件卸载或数据变化后最多 60s 内安全释放。
  const filePreviewURL = useMemo(() => {
    if (!filePreviewData || previewMode !== 'layout') return null;
    const url = URL.createObjectURL(filePreviewData);
    window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
    return url;
  }, [previewMode, filePreviewData]);

  // 下载始终走原文件接口；Office 的版式预览 PDF 不替代源文件下载。
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

  // 文档预览面向人工阅读：保留后端的安全分页，但在前端自动连续读取，不暴露工具游标和分页操作。
  useEffect(() => {
    if (shouldReadIndexedContent && hasNextContentPage && !isFetchingNextContentPage && !contentError) {
      void fetchNextContentPage();
    }
  }, [contentError, fetchNextContentPage, hasNextContentPage, isFetchingNextContentPage, shouldReadIndexedContent]);

  const layoutButtonLabel = isPdf ? '原始版式' : isImage ? '原图' : '版式';
  const contentButtonLabel = isImage ? 'OCR 正文' : '解析正文';

  return (
    <Paper variant="outlined" sx={{ p: 3, minHeight: 360, overflow: 'hidden' }}>
      <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>{document.title}</Typography>
        <Chip size="small" label={sourceLabel[document.source_type]} />
        <Chip
          size="small"
          color={effectiveStatus === 'failed' ? 'error' : effectiveStatus === 'succeeded' ? 'success' : 'info'}
          label={mainStatusLabel(effectiveStatus, document.index_mode)}
        />
        {canDownloadOriginal && (
          <Button
            size="small"
            variant="outlined"
            startIcon={<DownloadOutlined />}
            disabled={originalMutation.isPending}
            onClick={() => originalMutation.mutate()}
          >
            {originalMutation.isPending ? '正在下载…' : '下载原文件'}
          </Button>
        )}
        {document.source_type === 'url' && document.source_url && (
          <Button
            size="small"
            variant="outlined"
            startIcon={<OpenInNewOutlined />}
            component="a"
            href={document.source_url}
            target="_blank"
            rel="noopener noreferrer"
          >
            打开来源页面
          </Button>
        )}
        <Button size="small" variant="text" startIcon={<TuneOutlined />} onClick={() => setProcessingOpen(true)}>
          处理详情
        </Button>
      </Stack>
      {document.source_url && <Typography mt={1} variant="body2" color="text.secondary">来源：{document.source_url}</Typography>}
      {failureReason && <Alert severity="error" sx={{ mt: 2 }}>失败步骤：{failureStep || '未知'}；{failureReason}</Alert>}
      {originalMutation.error && <Alert severity="warning" sx={{ mt: 2 }}>原文件读取失败：{errorMessage(originalMutation.error)}</Alert>}

      <Divider sx={{ my: 2 }} />
      {hasFilePreview && (
        <Stack direction="row" spacing={1} mb={2}>
          <Button
            size="small"
            variant={previewMode === 'layout' ? 'contained' : 'outlined'}
            onClick={() => setPreviewMode('layout')}
          >
            {layoutButtonLabel}
          </Button>
          <Button
            size="small"
            variant={previewMode === 'content' ? 'contained' : 'outlined'}
            onClick={() => setPreviewMode('content')}
          >
            {contentButtonLabel}
          </Button>
        </Stack>
      )}
      {hasFilePreview && previewMode === 'layout' && filePreviewURL ? (
        // PDF 展示 MinIO 原文件；Office 展示转换后的 PDF；图片展示原图。
        isImage ? (
          <Box sx={{ textAlign: 'center' }}>
            <img
              src={filePreviewURL}
              alt={document.title}
              style={{ maxWidth: '100%', maxHeight: '72vh', objectFit: 'contain' }}
            />
          </Box>
        ) : (
          <Box sx={{ '& iframe': { width: '100%', height: '72vh', border: 'none', borderRadius: 1 } }}>
            <iframe src={filePreviewURL} title={isPdf ? 'PDF 原始版式预览' : 'Office 文档版式预览'} />
          </Box>
        )
      ) : hasFilePreview && previewMode === 'layout' && (renderedQuery.isPending || imageQuery.isPending) ? (
        <Stack spacing={1} py={5} alignItems="center">
          <Typography color="text.secondary">
            {isPdf ? '正在加载原始 PDF…' : isImage ? '正在加载原图…' : '正在生成 PDF 版式预览（首次需转换，可能稍慢）…'}
          </Typography>
        </Stack>
      ) : hasFilePreview && previewMode === 'layout' && (renderedQuery.isError || imageQuery.isError) ? (
        <>
          <Alert severity="warning" sx={{ mb: 2 }}>版式预览不可用：{errorMessage(isImage ? imageQuery.error : renderedQuery.error)}，已回退解析正文。</Alert>
          {renderTextPreview()}
        </>
      ) : (
        renderTextPreview()
      )}

      <DocumentProcessingDrawer
        open={processingOpen}
        onClose={() => setProcessingOpen(false)}
        document={document}
        processing={processing}
      />
    </Paper>
  );

  function renderTextPreview() {
    if (displayedContent) {
      return (
        <>
          {renderAsMarkdown ? (
            <Box sx={markdownSx}>
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                  img: ({ node, ...props }) => {
                    // ReactMarkdown 的 node 不是 img DOM 属性，不向下透传。
                    void node;
                    return (
                      // 点击图片在新标签页打开原图（asset URL 已带签名）。
                      <img
                        {...props}
                        onClick={() => {
                          const src = props.src;
                          if (src && !src.startsWith('data:')) window.open(src, '_blank', 'noopener');
                        }}
                      />
                    );
                  },
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
      );
    }
    if (previewQuery.isPending && shouldReadPreview) {
      return <Typography color="text.secondary">正在读取完整解析正文…</Typography>;
    }
    if (contentQuery.isPending && shouldReadIndexedContent) {
      return <Typography color="text.secondary">正在读取已索引正文…</Typography>;
    }
    if (contentQuery.error) {
      return <Alert severity="warning">正文读取失败：{errorMessage(contentQuery.error)}</Alert>;
    }
    return <Box py={5} textAlign="center"><Typography color="text.secondary">该文档暂时没有可显示的正文。</Typography></Box>;
  }
}
