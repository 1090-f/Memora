import NavigateBeforeOutlined from '@mui/icons-material/NavigateBeforeOutlined';
import NavigateNextOutlined from '@mui/icons-material/NavigateNextOutlined';
import ZoomInOutlined from '@mui/icons-material/ZoomInOutlined';
import ZoomOutOutlined from '@mui/icons-material/ZoomOutOutlined';
import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { GlobalWorkerOptions, getDocument, type PDFDocumentLoadingTask, type PDFDocumentProxy, type RenderTask } from 'pdfjs-dist';
import workerSrc from 'pdfjs-dist/build/pdf.worker.min.mjs?url';
import { useEffect, useRef, useState } from 'react';
import { errorMessage } from '@/api/errors';
import { getDocumentPreviewBlob } from '../../api';

GlobalWorkerOptions.workerSrc = workerSrc;

export function PdfViewer({ documentId, contentUrl, title }: { documentId: string; contentUrl: string; title: string }) {
  const blobQuery = useQuery({
    queryKey: ['documents', documentId, 'preview-pdf', contentUrl],
    queryFn: () => getDocumentPreviewBlob(contentUrl),
    staleTime: Infinity,
    retry: false,
  });
  const [pdf, setPdf] = useState<PDFDocumentProxy | null>(null);
  const [pageNumber, setPageNumber] = useState(1);
  const [scale, setScale] = useState(1.2);
  const [loadError, setLoadError] = useState<Error | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    if (!blobQuery.data) { setPdf(null); return; }
    let active = true;
    let loaded: PDFDocumentProxy | null = null;
    let task: PDFDocumentLoadingTask | null = null;
    void blobQuery.data.arrayBuffer().then((buffer) => {
      if (!active) return;
      task = getDocument({ data: buffer });
      return task.promise;
    }).then((document) => {
      if (!document) return;
      if (!active) { void document.destroy(); return; }
      loaded = document;
      setPdf(document);
      setPageNumber(1);
      setLoadError(null);
    }).catch((error: Error) => active && setLoadError(error));
    return () => {
      active = false;
      if (task) void task.destroy();
      if (loaded) void loaded.destroy();
    };
  }, [blobQuery.data]);

  useEffect(() => {
    if (!pdf || !canvasRef.current) return;
    let active = true;
    let renderTask: RenderTask | null = null;
    void pdf.getPage(pageNumber).then((page) => {
      if (!active || !canvasRef.current) return;
      const viewport = page.getViewport({ scale });
      const canvas = canvasRef.current;
      const context = canvas.getContext('2d');
      if (!context) return;
      const ratio = window.devicePixelRatio || 1;
      canvas.width = Math.floor(viewport.width * ratio);
      canvas.height = Math.floor(viewport.height * ratio);
      canvas.style.width = `${Math.floor(viewport.width)}px`;
      canvas.style.height = `${Math.floor(viewport.height)}px`;
      renderTask = page.render({ canvas, canvasContext: context, viewport, transform: ratio === 1 ? undefined : [ratio, 0, 0, ratio, 0, 0] });
      return renderTask.promise;
    }).catch((error: Error) => {
      if (active && error.name !== 'RenderingCancelledException') setLoadError(error);
    });
    return () => { active = false; renderTask?.cancel(); };
  }, [pageNumber, pdf, scale]);

  if (blobQuery.isPending) return <Typography color="text.secondary">正在加载 PDF…</Typography>;
  if (blobQuery.error) return <Alert severity="warning">PDF 读取失败：{errorMessage(blobQuery.error)}</Alert>;
  if (loadError) return <Alert severity="warning">PDF 解析失败：{loadError.message}</Alert>;
  if (!pdf) return <Typography color="text.secondary">正在解析 PDF…</Typography>;

  return (
    <Stack spacing={1.5}>
      <Stack direction="row" alignItems="center" justifyContent="center" spacing={1} flexWrap="wrap">
        <Button size="small" startIcon={<NavigateBeforeOutlined />} disabled={pageNumber <= 1} onClick={() => setPageNumber((value) => Math.max(1, value - 1))}>上一页</Button>
        <Typography variant="body2">第 {pageNumber} / {pdf.numPages} 页</Typography>
        <Button size="small" endIcon={<NavigateNextOutlined />} disabled={pageNumber >= pdf.numPages} onClick={() => setPageNumber((value) => Math.min(pdf.numPages, value + 1))}>下一页</Button>
        <Button size="small" startIcon={<ZoomOutOutlined />} disabled={scale <= 0.6} onClick={() => setScale((value) => Math.max(0.6, Number((value - 0.2).toFixed(1))))}>缩小</Button>
        <Button size="small" startIcon={<ZoomInOutlined />} disabled={scale >= 2.4} onClick={() => setScale((value) => Math.min(2.4, Number((value + 0.2).toFixed(1))))}>放大</Button>
      </Stack>
      <Box aria-label={`${title} PDF 页面`} sx={{ maxHeight: '72vh', overflow: 'auto', bgcolor: 'grey.100', p: 2, textAlign: 'center' }}>
        <canvas ref={canvasRef} style={{ display: 'inline-block', background: '#fff', boxShadow: '0 2px 12px rgba(0,0,0,.16)' }} />
      </Box>
    </Stack>
  );
}
