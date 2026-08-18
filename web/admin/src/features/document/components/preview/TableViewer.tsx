import NavigateBeforeOutlined from '@mui/icons-material/NavigateBeforeOutlined';
import NavigateNextOutlined from '@mui/icons-material/NavigateNextOutlined';
import FullscreenOutlined from '@mui/icons-material/FullscreenOutlined';
import GridViewOutlined from '@mui/icons-material/GridViewOutlined';
import InfoOutlined from '@mui/icons-material/InfoOutlined';
import TableChartOutlined from '@mui/icons-material/TableChartOutlined';
import { Alert, Box, Button, IconButton, Stack, Tab, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Tabs, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { errorMessage } from '@/api/errors';
import { getDocumentTablePreview } from '../../api';

const PAGE_SIZE = 200;

function columnLabel(column: number) {
  let value = column + 1;
  let label = '';
  while (value > 0) {
    const remainder = (value - 1) % 26;
    label = String.fromCharCode(65 + remainder) + label;
    value = Math.floor((value - 1) / 26);
  }
  return label;
}

export function TableViewer({ documentId, contentUrl }: { documentId: string; contentUrl: string }) {
  const [sheetIndex, setSheetIndex] = useState(0);
  const [rowOffset, setRowOffset] = useState(0);
  const [fullscreen, setFullscreen] = useState(false);
  const query = useQuery({
    queryKey: ['documents', documentId, 'preview-table', contentUrl, sheetIndex, rowOffset],
    queryFn: () => getDocumentTablePreview(contentUrl, { sheet_index: sheetIndex, row_offset: rowOffset, row_limit: PAGE_SIZE }),
    retry: false,
  });

  useEffect(() => { setSheetIndex(0); setRowOffset(0); }, [documentId, contentUrl]);
  const activeSheet = query.data?.sheets.find((sheet) => sheet.index === sheetIndex);
  const maxColumns = activeSheet?.column_count ?? 0;
  const rows = useMemo(() => query.data?.rows ?? [], [query.data?.rows]);
  const { mergeAnchors, coveredCells } = useMemo(() => {
    const anchors = new Map<string, { rowSpan: number; colSpan: number }>();
    const covered = new Set<string>();
    const pageEnd = rowOffset + rows.length;
    for (const merge of query.data?.merged_cells ?? []) {
      if (merge.start_row < rowOffset || merge.start_row >= pageEnd) continue;
      const rowSpan = Math.min(merge.row_span, pageEnd - merge.start_row);
      const colSpan = Math.min(merge.column_span, maxColumns - merge.start_column);
      if (rowSpan < 1 || colSpan < 1) continue;
      anchors.set(`${merge.start_row}:${merge.start_column}`, { rowSpan, colSpan });
      for (let row = merge.start_row; row < merge.start_row + rowSpan; row += 1) {
        for (let column = merge.start_column; column < merge.start_column + colSpan; column += 1) {
          if (row !== merge.start_row || column !== merge.start_column) covered.add(`${row}:${column}`);
        }
      }
    }
    return { mergeAnchors: anchors, coveredCells: covered };
  }, [maxColumns, query.data?.merged_cells, rowOffset, rows.length]);

  if (query.isPending) return <Typography color="text.secondary">正在读取工作表…</Typography>;
  if (query.error) return <Alert severity="warning">工作表读取失败：{errorMessage(query.error)}</Alert>;
  if (!query.data) return null;

  return (
    <Stack spacing={1.8} sx={fullscreen ? { position: 'fixed', inset: 16, zIndex: 1400, p: 3, overflow: 'auto', bgcolor: '#fff' } : undefined}>
      <Stack direction={{ xs: 'column', sm: 'row' }} alignItems={{ sm: 'center' }} justifyContent="space-between" gap={1}>
        <Tabs value={sheetIndex} onChange={(_, value: number) => { setSheetIndex(value); setRowOffset(0); }} variant="scrollable" scrollButtons="auto" sx={{ minHeight: 50, '& .MuiTabs-indicator': { height: 2, bgcolor: '#5458f4' }, '& .MuiTab-root': { minHeight: 50, minWidth: 112, borderRadius: 1.25, color: '#68738a', fontWeight: 650 }, '& .Mui-selected': { color: '#5054ec !important', bgcolor: '#f4f4ff' } }}>
          {query.data.sheets.map((sheet) => <Tab key={sheet.index} value={sheet.index} label={sheet.name} />)}
        </Tabs>
        <Stack direction="row" alignItems="center" spacing={0.75} sx={{ flexShrink: 0 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', p: 0.35, border: '1px solid #edf0f6', borderRadius: 1.4, bgcolor: '#fafbff' }}>
            <Button size="small" startIcon={<GridViewOutlined fontSize="small" />} sx={{ height: 36, borderRadius: 1, color: '#5155eb', bgcolor: '#fff', boxShadow: '0 1px 4px rgba(46, 54, 102, .08)', fontWeight: 650 }}>单元格视图</Button>
            <Button size="small" startIcon={<TableChartOutlined fontSize="small" />} sx={{ height: 36, color: '#6d7587', fontWeight: 600 }}>表格视图</Button>
          </Box>
          <IconButton aria-label={fullscreen ? '退出全屏预览' : '全屏预览'} onClick={() => setFullscreen((current) => !current)} sx={{ width: 44, height: 44, border: '1px solid #e5e9f0', borderRadius: 1.4, color: '#536078' }}><FullscreenOutlined /></IconButton>
        </Stack>
      </Stack>
      <TableContainer
        sx={{
          maxHeight: fullscreen ? 'calc(100vh - 210px)' : '50vh',
          maxWidth: '100%',
          overflowX: 'auto',
          overflowY: 'auto',
          border: '1px solid #e3e7ee',
          borderRadius: 1.5,
          '&::-webkit-scrollbar': { height: 8, width: 8 },
          '&::-webkit-scrollbar-track': { bgcolor: 'transparent' },
          '&::-webkit-scrollbar-thumb': { bgcolor: 'grey.400', borderRadius: 4 },
          '&::-webkit-scrollbar-thumb:hover': { bgcolor: 'grey.500' },
          scrollbarWidth: 'thin',
          scrollbarColor: '#b0b0b0 transparent',
        }}
      >
        <Table size="small" stickyHeader style={{ width: 'max-content', minWidth: `${Math.max(maxColumns * 150 + 82, 100)}px` }}>
          <TableHead>
            <TableRow>
              <TableCell sx={{ position: 'sticky', left: 0, zIndex: 4, minWidth: 82, bgcolor: '#fafbfe', borderColor: '#e6e9f0' }} />
              {Array.from({ length: maxColumns }, (_, column) => <TableCell key={column} align="center" sx={{ minWidth: 150, height: 52, bgcolor: '#fafbfe', borderColor: '#e6e9f0', color: '#565bf0', fontWeight: 650 }}>{columnLabel(column)}</TableCell>)}
            </TableRow>
          </TableHead>
          <TableBody>
            {rows.map((row) => {
              const cells = new Map(row.cells.map((cell) => [cell.column, cell.value]));
              return (
                <TableRow key={row.row}>
                  <TableCell align="center" sx={{ position: 'sticky', left: 0, zIndex: 2, bgcolor: '#fafbfe', color: '#546078', minWidth: 82, borderColor: '#e6e9f0', fontWeight: 600 }}>{row.row + 1}</TableCell>
                  {Array.from({ length: maxColumns }, (_, column) => {
                    const key = `${row.row}:${column}`;
                    if (coveredCells.has(key)) return null;
                    const merge = mergeAnchors.get(key);
                    const value = cells.get(column) ?? '';
                    return (
                      <TableCell
                        key={column}
                        title={value}
                        rowSpan={merge?.rowSpan}
                        colSpan={merge?.colSpan}
                        onDoubleClick={() => void navigator.clipboard?.writeText(value)}
                        sx={{ minWidth: 150, height: 64, borderColor: '#e6e9f0', whiteSpace: 'nowrap', textAlign: 'center', color: '#303849' }}
                      >
                        {value}
                      </TableCell>
                    );
                  })}
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </TableContainer>
      <Stack direction={{ xs: 'column', sm: 'row' }} alignItems={{ sm: 'center' }} justifyContent="space-between" gap={1}>
        <Typography sx={{ fontSize: 14, color: '#68748b' }}>
          {rows.length > 0 ? `共 ${activeSheet?.row_count ?? rows.length} 行，${maxColumns} 列` : '暂无数据'}（双击单元格编辑）
        </Typography>
        <Stack direction="row" alignItems="center" spacing={0.4} sx={{ color: '#778197' }}>
          <IconButton size="small" aria-label="上一页" disabled={rowOffset === 0} onClick={() => setRowOffset(Math.max(0, rowOffset - PAGE_SIZE))}><NavigateBeforeOutlined /></IconButton>
          <Box sx={{ minWidth: 72, height: 40, display: 'grid', placeItems: 'center', borderRadius: 1.25, bgcolor: '#f5f5ff', color: '#5155ed', fontWeight: 700 }}>1</Box>
          <Typography sx={{ px: 0.6, color: '#596276' }}>/ 1</Typography>
          <IconButton size="small" aria-label="下一页" disabled={query.data.next_row_offset === undefined} onClick={() => query.data?.next_row_offset !== undefined && setRowOffset(query.data.next_row_offset)}><NavigateNextOutlined /></IconButton>
        </Stack>
      </Stack>
      <Alert icon={<InfoOutlined fontSize="inherit" />} severity="info" variant="outlined" sx={{ borderColor: '#cbd9ff', bgcolor: '#f9fbff', color: '#4f79dd', '& .MuiAlert-icon': { color: '#6991ed' } }}>提示：预览内容为部分数据展示，下载原文件可查看完整内容</Alert>
    </Stack>
  );
}
