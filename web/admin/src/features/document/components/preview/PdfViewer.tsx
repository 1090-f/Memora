import NavigateBeforeOutlined from '@mui/icons-material/NavigateBeforeOutlined';
import NavigateNextOutlined from '@mui/icons-material/NavigateNextOutlined';
import ZoomInOutlined from '@mui/icons-material/ZoomInOutlined';
import ZoomOutOutlined from '@mui/icons-material/ZoomOutOutlined';
import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { GlobalWorkerOptions, getDocument, type PDFDocumentLoadingTask, type PDFDocumentProxy, type RenderTask } from 'pdfjs-dist';
import workerSrc from 'pdfjs-dist/build/pdf.worker.min.mjs?url';
import { useEffect, useRef, useState, type WheelEvent } from 'react';
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
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const wheelDeltaRef = useRef(0);
  const wheelGestureActiveRef = useRef(false);
  const wheelEndTimerRef = useRef<number | null>(null);
  const wheelPaging = /\.pptx$/i.test(title);

  const handleWheel = (event: WheelEvent<HTMLDivElement>) => {
    if (!wheelPaging || !pdf || event.ctrlKey || event.metaKey || Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;

    const viewport = event.currentTarget;
    const verticalDelta = event.deltaMode === 1
      ? event.deltaY * 16
      : event.deltaMode === 2 ? event.deltaY * viewport.clientHeight : event.deltaY;
    const scrollingDown = verticalDelta > 0;
    const hasVerticalOverflow = viewport.scrollHeight > viewport.clientHeight + 1;
    const atVerticalEdge = scrollingDown
      ? viewport.scrollTop + viewport.clientHeight >= viewport.scrollHeight - 1
      : viewport.scrollTop <= 1;
    if (hasVerticalOverflow && !atVerticalEdge) return;

    const canChangePage = scrollingDown ? pageNumber < pdf.numPages : pageNumber > 1;
    if (!canChangePage) return;
    event.preventDefault();

    if (wheelEndTimerRef.current !== null) window.clearTimeout(wheelEndTimerRef.current);
    wheelEndTimerRef.current = window.setTimeout(() => {
      wheelDeltaRef.current = 0;
      wheelGestureActiveRef.current = false;
      wheelEndTimerRef.current = null;
    }, 180);
    if (wheelGestureActiveRef.current) return;

    if (wheelDeltaRef.current !== 0 && Math.sign(wheelDeltaRef.current) !== Math.sign(verticalDelta)) {
      wheelDeltaRef.current = 0;
    }
    wheelDeltaRef.current += verticalDelta;
    if (Math.abs(wheelDeltaRef.current) < 40) return;

    wheelGestureActiveRef.current = true;
    wheelDeltaRef.current = 0;
    setPageNumber((value) => scrollingDown ? Math.min(pdf.numPages, value + 1) : Math.max(1, value - 1));
  };

  useEffect(() => {
    if (!blobQuery.data) { setPdf(null); return; }
    let active = true;
    let task: PDFDocumentLoadingTask | null = null;
    void blobQuery.data.arrayBuffer().then((buffer) => {
      if (!active) return;
      task = getDocument({ data: buffer });
      return task.promise;
    }).then((document) => {
      if (!document) return;
      if (!active) return;
      setPdf(document);
      setPageNumber(1);
      setLoadError(null);
    }).catch((error: Error) => active && setLoadError(error));
    return () => {
      active = false;
      if (task) void task.destroy();
    };
  }, [blobQuery.data]);

  useEffect(() => {
    if (!pdf || !canvasRef.current) return;
    let active = true;
    let renderTask: RenderTask | null = null;
    void pdf.getPage(pageNumber).then((page) => {
      if (!active) return;
      const viewport = page.getViewport({ scale });
      const nextCanvas = window.document.createElement('canvas');
      const context = nextCanvas.getContext('2d');
      if (!context) return;
      const ratio = window.devicePixelRatio || 1;
      nextCanvas.width = Math.floor(viewport.width * ratio);
      nextCanvas.height = Math.floor(viewport.height * ratio);
      renderTask = page.render({ canvas: nextCanvas, canvasContext: context, viewport, transform: ratio === 1 ? undefined : [ratio, 0, 0, ratio, 0, 0] });
      return renderTask.promise.then(() => {
        if (!active || !canvasRef.current) return;
        const canvas = canvasRef.current;
        const visibleContext = canvas.getContext('2d');
        if (!visibleContext) return;

        canvas.width = nextCanvas.width;
        canvas.height = nextCanvas.height;
        canvas.style.width = `${Math.floor(viewport.width)}px`;
        canvas.style.height = `${Math.floor(viewport.height)}px`;
        visibleContext.drawImage(nextCanvas, 0, 0);
        if (viewportRef.current) viewportRef.current.scrollTop = 0;
      });
    }).catch((error: Error) => {
      if (active && error.name !== 'RenderingCancelledException') setLoadError(error);
    });
    return () => { active = false; renderTask?.cancel(); };
  }, [pageNumber, pdf, scale]);

  useEffect(() => () => {
    if (wheelEndTimerRef.current !== null) window.clearTimeout(wheelEndTimerRef.current);
  }, []);

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
        {wheelPaging && <Typography variant="caption" color="text.secondary">滚轮翻页</Typography>}
      </Stack>
      <Box ref={viewportRef} aria-label={`${title} PDF 页面`} onWheel={handleWheel} sx={{ maxHeight: '72vh', overflow: 'auto', bgcolor: 'grey.100', p: 2, textAlign: 'center' }}>
        <canvas ref={canvasRef} style={{ display: 'inline-block', background: '#fff', boxShadow: '0 2px 12px rgba(0,0,0,.16)' }} />
      </Box>
    </Stack>
  );
}
