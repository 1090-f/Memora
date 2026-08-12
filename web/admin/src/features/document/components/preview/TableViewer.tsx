import NavigateBeforeOutlined from '@mui/icons-material/NavigateBeforeOutlined';
import NavigateNextOutlined from '@mui/icons-material/NavigateNextOutlined';
import { Alert, Box, Button, Stack, Tab, Table, TableBody, TableCell, TableContainer, TableRow, Tabs, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { errorMessage } from '@/api/errors';
import { getDocumentTablePreview } from '../../api';

const PAGE_SIZE = 200;

export function TableViewer({ documentId, contentUrl }: { documentId: string; contentUrl: string }) {
  const [sheetIndex, setSheetIndex] = useState(0);
  const [rowOffset, setRowOffset] = useState(0);
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
    <Stack spacing={1.5}>
      <Tabs value={sheetIndex} onChange={(_, value: number) => { setSheetIndex(value); setRowOffset(0); }} variant="scrollable" scrollButtons="auto">
        {query.data.sheets.map((sheet) => <Tab key={sheet.index} value={sheet.index} label={sheet.name} />)}
      </Tabs>
      <TableContainer sx={{ maxHeight: '66vh', border: 1, borderColor: 'divider' }}>
        <Table size="small" stickyHeader sx={{ width: 'max-content', minWidth: '100%' }}>
          <TableBody>
            {rows.map((row) => {
              const cells = new Map(row.cells.map((cell) => [cell.column, cell.value]));
              return (
                <TableRow key={row.row}>
                  <TableCell sx={{ position: 'sticky', left: 0, zIndex: 2, bgcolor: 'background.paper', color: 'text.secondary', minWidth: 64 }}>{row.row + 1}</TableCell>
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
                        sx={{ minWidth: 120, maxWidth: 320, whiteSpace: 'pre-wrap' }}
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
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography variant="caption" color="text.secondary">
          {rows.length > 0 ? `第 ${rowOffset + 1}～${rowOffset + rows.length} 行` : '暂无数据'}；双击单元格复制
        </Typography>
        <Box>
          <Button size="small" startIcon={<NavigateBeforeOutlined />} disabled={rowOffset === 0} onClick={() => setRowOffset(Math.max(0, rowOffset - PAGE_SIZE))}>上一页</Button>
          <Button size="small" endIcon={<NavigateNextOutlined />} disabled={query.data.next_row_offset === undefined} onClick={() => query.data?.next_row_offset !== undefined && setRowOffset(query.data.next_row_offset)}>下一页</Button>
        </Box>
      </Stack>
    </Stack>
  );
}
