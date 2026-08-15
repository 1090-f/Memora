import NavigateBeforeOutlined from '@mui/icons-material/NavigateBeforeOutlined';
import NavigateNextOutlined from '@mui/icons-material/NavigateNextOutlined';
import { Alert, Box, Button, Stack, Tab, Table, TableBody, TableCell, TableContainer, TableRow, Tabs, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import { errorMessage } from '@/api/errors';
import { getOriginalDocument } from '../../api';
import type { BrowserOfficeType } from './browserOfficeType';
import { isZipArchive, validateOfficeArchive } from './officeArchiveSafety';

const MAX_BROWSER_FILE_BYTES = 64 * 1024 * 1024;
const SPREADSHEET_PAGE_SIZE = 200;
const MAX_SPREADSHEET_ROWS = 10_000;

export function BrowserOfficeViewer({ documentId, type, fileSize }: { documentId: string; type: BrowserOfficeType; fileSize?: number }) {
  const tooLarge = fileSize !== undefined && fileSize > MAX_BROWSER_FILE_BYTES;
  const query = useQuery({
    queryKey: ['documents', documentId, 'original-browser-preview'],
    queryFn: () => getOriginalDocument(documentId, true),
    staleTime: Infinity,
    retry: false,
    enabled: !tooLarge,
  });

  if (tooLarge) return <Alert severity="warning">文件超过 64 MB，为避免浏览器卡顿，已停止本地预览。请下载原文件查看。</Alert>;
  if (query.isPending) return <Typography color="text.secondary">正在读取原文件…</Typography>;
  if (query.error) return <Alert severity="warning">原文件读取失败：{errorMessage(query.error)}</Alert>;
  if (!query.data) return <Typography color="text.secondary">没有可读取的原文件。</Typography>;
  if (query.data.size > MAX_BROWSER_FILE_BYTES) {
    return <Alert severity="warning">文件超过 64 MB，为避免浏览器卡顿，已停止本地预览。请下载原文件查看。</Alert>;
  }

  switch (type) {
    case 'docx': return <DocxViewer blob={query.data} />;
    case 'pptx': return <PptxViewer blob={query.data} />;
    case 'spreadsheet': return <SpreadsheetViewer blob={query.data} />;
  }
}

function DocxViewer({ blob }: { blob: Blob }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [rendering, setRendering] = useState(true);
  const [renderError, setRenderError] = useState<unknown>();

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const bodyHost = window.document.createElement('div');
    const styleHost = window.document.createElement('div');
    container.replaceChildren(styleHost, bodyHost);
    let active = true;
    setRendering(true);
    setRenderError(undefined);
    void (async () => {
      try {
        const [{ renderAsync }, buffer] = await Promise.all([import('docx-preview'), blob.arrayBuffer()]);
        validateOfficeArchive(buffer);
        await renderAsync(buffer, bodyHost, styleHost, {
          breakPages: true,
          ignoreHeight: false,
          ignoreWidth: false,
          renderAltChunks: false,
          renderComments: false,
          useBase64URL: true,
        });
        if (active) setRendering(false);
      } catch (error) {
        if (active) {
          setRendering(false);
          setRenderError(error);
        }
      }
    })();
    return () => {
      active = false;
      bodyHost.remove();
      styleHost.remove();
    };
  }, [blob]);

  return (
    <Stack spacing={1}>
      {rendering && <Typography color="text.secondary">正在浏览器中解析 DOCX…</Typography>}
      {renderError !== undefined && <PreviewFailure format="DOCX" error={renderError} />}
      <Box
        ref={containerRef}
        sx={{
          maxHeight: '72vh', overflow: 'auto', bgcolor: '#e8eaed', border: 1, borderColor: 'divider', borderRadius: 1,
          '& .docx-wrapper': { bgcolor: '#e8eaed', p: { xs: 1, md: 2 } },
          '& .docx': { maxWidth: '100%', boxShadow: 2 },
        }}
      />
    </Stack>
  );
}

function PptxViewer({ blob }: { blob: Blob }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [rendering, setRendering] = useState(true);
  const [renderError, setRenderError] = useState<unknown>();

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const host = window.document.createElement('div');
    host.style.minHeight = '420px';
    container.replaceChildren(host);
    const abortController = new AbortController();
    let active = true;
    let viewer: { destroy: () => void } | undefined;
    setRendering(true);
    setRenderError(undefined);
    void (async () => {
      try {
        const [{ PptxViewer: PptxRenderer, RECOMMENDED_ZIP_LIMITS }, buffer] = await Promise.all([
          import('@aiden0z/pptx-renderer'),
          blob.arrayBuffer(),
        ]);
        if (!active) return;
        const nextViewer = await PptxRenderer.open(buffer, host, {
          zipLimits: RECOMMENDED_ZIP_LIMITS,
          lazyMedia: true,
          lazySlides: true,
          listOptions: { windowed: true, initialSlides: 4, batchSize: 4 },
          pdfjs: false,
          signal: abortController.signal,
        });
        if (!active) {
          nextViewer.destroy();
          return;
        }
        viewer = nextViewer;
        setRendering(false);
      } catch (error) {
        if (active && !abortController.signal.aborted) {
          setRendering(false);
          setRenderError(error);
        }
      }
    })();
    return () => {
      active = false;
      abortController.abort();
      viewer?.destroy();
      host.remove();
    };
  }, [blob]);

  return (
    <Stack spacing={1}>
      {rendering && <Typography color="text.secondary">正在浏览器中解析 PPTX…</Typography>}
      {renderError !== undefined && <PreviewFailure format="PPTX" error={renderError} />}
      <Box ref={containerRef} sx={{ maxHeight: '72vh', overflow: 'auto', bgcolor: '#25272b', border: 1, borderColor: 'divider', borderRadius: 1, p: 1 }} />
    </Stack>
  );
}

type XLSXModule = typeof import('xlsx');
type XLSXWorkbook = import('xlsx').WorkBook;

function SpreadsheetViewer({ blob }: { blob: Blob }) {
  const [parsed, setParsed] = useState<{ xlsx: XLSXModule; workbook: XLSXWorkbook }>();
  const [renderError, setRenderError] = useState<unknown>();
  const [sheetIndex, setSheetIndex] = useState(0);
  const [rowOffset, setRowOffset] = useState(0);

  useEffect(() => {
    let active = true;
    setParsed(undefined);
    setRenderError(undefined);
    setSheetIndex(0);
    setRowOffset(0);
    void (async () => {
      try {
        const [xlsx, buffer] = await Promise.all([import('xlsx'), blob.arrayBuffer()]);
        if (isZipArchive(buffer)) validateOfficeArchive(buffer);
        const workbook = xlsx.read(buffer, { type: 'array', cellDates: true, sheetRows: MAX_SPREADSHEET_ROWS });
        if (active) setParsed({ xlsx, workbook });
      } catch (error) {
        if (active) setRenderError(error);
      }
    })();
    return () => { active = false; };
  }, [blob]);

  const rows = useMemo(() => {
    if (!parsed) return [] as unknown[][];
    const sheetName = parsed.workbook.SheetNames[sheetIndex];
    const sheet = sheetName ? parsed.workbook.Sheets[sheetName] : undefined;
    return sheet ? parsed.xlsx.utils.sheet_to_json<unknown[]>(sheet, { header: 1, defval: '', raw: false }) : [];
  }, [parsed, sheetIndex]);
  const pageRows = rows.slice(rowOffset, rowOffset + SPREADSHEET_PAGE_SIZE);
  const maxColumns = pageRows.reduce((maximum, row) => Math.max(maximum, row.length), 0);

  if (renderError) return <PreviewFailure format="表格" error={renderError} />;
  if (!parsed) return <Typography color="text.secondary">正在浏览器中解析工作簿…</Typography>;

  return (
    <Stack spacing={1.5}>
      <Alert severity="info">浏览器表格预览最多读取每个工作表前 {MAX_SPREADSHEET_ROWS.toLocaleString()} 行，完整内容请下载原文件。</Alert>
      <Tabs value={sheetIndex} onChange={(_, value: number) => { setSheetIndex(value); setRowOffset(0); }} variant="scrollable" scrollButtons="auto">
        {parsed.workbook.SheetNames.map((name, index) => <Tab key={`${index}-${name}`} value={index} label={name} />)}
      </Tabs>
      <TableContainer sx={{ maxHeight: '66vh', border: 1, borderColor: 'divider' }}>
        <Table size="small" stickyHeader sx={{ width: 'max-content', minWidth: '100%' }}>
          <TableBody>
            {pageRows.map((row, pageIndex) => (
              <TableRow key={rowOffset + pageIndex}>
                <TableCell sx={{ position: 'sticky', left: 0, zIndex: 2, bgcolor: 'background.paper', color: 'text.secondary', minWidth: 64 }}>
                  {rowOffset + pageIndex + 1}
                </TableCell>
                {Array.from({ length: maxColumns }, (_, column) => {
                  const value = formatCell(row[column]);
                  return (
                    <TableCell
                      key={column}
                      title={value}
                      onDoubleClick={() => void navigator.clipboard?.writeText(value)}
                      sx={{ minWidth: 120, maxWidth: 320, whiteSpace: 'pre-wrap' }}
                    >
                      {value}
                    </TableCell>
                  );
                })}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="caption" color="text.secondary">
          {pageRows.length > 0 ? `第 ${rowOffset + 1}～${rowOffset + pageRows.length} 行，共 ${rows.length} 行` : '暂无数据'}；双击单元格复制
        </Typography>
        <Box>
          <Button size="small" startIcon={<NavigateBeforeOutlined />} disabled={rowOffset === 0} onClick={() => setRowOffset(Math.max(0, rowOffset - SPREADSHEET_PAGE_SIZE))}>上一页</Button>
          <Button size="small" endIcon={<NavigateNextOutlined />} disabled={rowOffset + SPREADSHEET_PAGE_SIZE >= rows.length} onClick={() => setRowOffset(rowOffset + SPREADSHEET_PAGE_SIZE)}>下一页</Button>
        </Box>
      </Stack>
    </Stack>
  );
}

function formatCell(value: unknown) {
  if (value === null || value === undefined) return '';
  if (value instanceof Date) return value.toLocaleString();
  return String(value);
}

function PreviewFailure({ format, error }: { format: string; error: unknown }) {
  return (
    <Alert severity="warning">
      {format} 浏览器预览失败：{errorMessage(error)}。可以切换到解析正文，或下载原文件查看。
    </Alert>
  );
}
