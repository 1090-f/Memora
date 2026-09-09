import { Alert, Box, CircularProgress, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { errorMessage } from '@/api/errors';
import { getDocumentPreviewBlob } from '../../api';

type OfficePreviewType = 'docx' | 'pptx';
type RenderState = 'idle' | 'rendering' | 'ready' | 'failed';
type PptxViewer = { preview(data: ArrayBuffer): Promise<void> | void; destroy?: () => void };

export function OfficeViewer({ documentId, contentUrl, type }: {
  documentId: string;
  contentUrl: string;
  type: OfficePreviewType;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [renderState, setRenderState] = useState<RenderState>('idle');
  const [renderError, setRenderError] = useState('');
  const query = useQuery({
    queryKey: ['documents', documentId, 'preview-office-source', contentUrl],
    queryFn: () => getDocumentPreviewBlob(contentUrl),
    retry: false,
    staleTime: 5 * 60_000,
    gcTime: 60_000,
  });

  useEffect(() => {
    const container = containerRef.current;
    if (!container || !query.data) return;

    let active = true;
    let timedOut = false;
    let pptxViewer: PptxViewer | undefined;
    container.replaceChildren();
    setRenderState('rendering');
    setRenderError('');

    const timeoutId = window.setTimeout(() => {
      timedOut = true;
      if (!active) return;
      setRenderError('浏览器渲染超时，请查看解析正文、PDF 预览或下载原文件。');
      setRenderState('failed');
    }, 30_000);

    const render = async () => {
      const renderHost = window.document.createElement('div');
      renderHost.className = type === 'docx' ? 'office-docx-host' : 'office-pptx-host';
      container.replaceChildren(renderHost);

      if (type === 'docx') {
        const { renderAsync } = await import('docx-preview');
        if (!active) return;
        await renderAsync(query.data, renderHost, renderHost, {
          className: 'docx',
          inWrapper: true,
          ignoreWidth: false,
          ignoreHeight: false,
          ignoreFonts: false,
          breakPages: true,
          experimental: true,
        });
      } else {
        const [buffer, module] = await Promise.all([query.data.arrayBuffer(), import('pptx-preview')]);
        if (!active) return;
        pptxViewer = module.init(renderHost, { width: 960, height: 540 }) as PptxViewer;
        await pptxViewer.preview(buffer);
      }

      if (active && !timedOut) setRenderState('ready');
    };

    void render().catch((reason: unknown) => {
      if (!active || timedOut) return;
      setRenderError(errorMessage(reason));
      setRenderState('failed');
    }).finally(() => {
      window.clearTimeout(timeoutId);
    });

    return () => {
      active = false;
      window.clearTimeout(timeoutId);
      try {
        pptxViewer?.destroy?.();
      } finally {
        container.replaceChildren();
      }
    };
  }, [query.data, type]);

  if (query.isPending) {
    return <Typography color="text.secondary">正在读取原文件…</Typography>;
  }
  if (query.error) {
    return <Alert severity="warning">原文件读取失败：{errorMessage(query.error)}</Alert>;
  }

  return (
    <Box>
      {renderState === 'rendering' && (
        <Stack direction="row" spacing={1.2} alignItems="center" sx={{ mb: 2 }}>
          <CircularProgress size={18} />
          <Typography color="text.secondary">正在浏览器中渲染文档…</Typography>
        </Stack>
      )}
      {renderState === 'failed' && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          文件预览失败：{renderError || '浏览器无法渲染该文档，请查看解析正文、PDF 预览或下载原文件。'}
        </Alert>
      )}
      <Box
        ref={containerRef}
        sx={{
          display: renderState === 'failed' ? 'none' : 'block',
          minHeight: 420,
          overflow: 'auto',
          borderRadius: 2,
          bgcolor: '#eef1f5',
          '& .office-docx-host': { minWidth: 'fit-content' },
          '& .docx-wrapper': { bgcolor: '#eef1f5', py: 2.5, px: 1.5 },
          '& .docx-wrapper > section.docx': { boxShadow: '0 8px 24px rgba(31, 42, 68, .14)' },
          '& .office-pptx-host': { width: 960, minHeight: 540, mx: 'auto', bgcolor: '#fff' },
        }}
      />
    </Box>
  );
}
