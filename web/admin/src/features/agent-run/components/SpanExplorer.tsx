import { useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  Paper,
  Stack,
  Typography,
} from '@mui/material';
import type { TraceSpan } from '../types';
import { buildTraceSpanRows, traceDurationMS } from '../spanTree';

function formatDuration(value: number) {
  if (value >= 1000) return `${(value / 1000).toFixed(2)} s`;
  if (value > 0 && value < 1) return '< 1 ms';
  return `${value} ms`;
}

function spanCategory(span: TraceSpan) {
  const name = span.name.toLowerCase();
  if (name.startsWith('db.') || span.attributes['db.system.name']) return { label: '数据库', color: '#7c3aed' };
  if (name.includes('queue') || span.attributes['messaging.system']) return { label: '队列', color: '#14b8a6' };
  if (name.startsWith('tool.')) return { label: '工具', color: '#16a34a' };
  if (name.startsWith('context.')) return { label: '上下文', color: '#9333ea' };
  if (name.includes('model') || name.includes('llm') || span.attributes['gen_ai.request.model']) return { label: '模型', color: '#2563eb' };
  if (name.includes('agent')) return { label: 'Agent', color: '#f59e0b' };
  if (name.includes('http') || span.attributes['http.request.method']) return { label: 'HTTP', color: '#64748b' };
  return { label: '内部', color: '#5b5bd6' };
}

function JsonBlock({ value }: { value: unknown }) {
  return (
    <Box component="pre" sx={{ m: 0, p: 1.5, maxHeight: 240, overflow: 'auto', borderRadius: 1, bgcolor: '#f7f8fb', fontSize: 12, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
      {JSON.stringify(value, null, 2)}
    </Box>
  );
}

export function SpanExplorer({ spans, loading, error }: { spans: TraceSpan[]; loading?: boolean; error?: boolean }) {
  const [selectedID, setSelectedID] = useState<string>();
  const [onlyErrors, setOnlyErrors] = useState(false);
  const [showDatabase, setShowDatabase] = useState(false);
  const visible = useMemo(() => spans.filter((span) => {
    if (!showDatabase && spanCategory(span).label === '数据库') return false;
    return !onlyErrors || span.status_code.toLowerCase() === 'error';
  }), [onlyErrors, showDatabase, spans]);
  const rows = useMemo(() => buildTraceSpanRows(visible), [visible]);
  const selected = visible.find((span) => span.span_id === selectedID) ?? rows[0]?.span;

  if (loading && spans.length === 0) return <Stack direction="row" spacing={1} alignItems="center"><CircularProgress size={18} /><Typography color="text.secondary">正在加载技术链路…</Typography></Stack>;
  if (error) return <Alert severity="warning">技术链路加载失败，业务阶段记录仍可正常查看。</Alert>;
  if (spans.length === 0) return <Alert severity="info">暂无持久化 Span。旧运行或服务升级前创建的记录不会自动生成技术链路。</Alert>;

  return (
    <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ sm: 'center' }} sx={{ px: 2, py: 1.5 }}>
        <Box sx={{ flex: 1 }}>
          <Typography fontWeight={750}>Trace Explorer</Typography>
          <Typography variant="caption" color="text.secondary">{spans.length} 个 Span · 总跨度 {formatDuration(traceDurationMS(spans))}</Typography>
        </Box>
        <Button size="small" variant={showDatabase ? 'contained' : 'outlined'} onClick={() => setShowDatabase((value) => !value)}>{showDatabase ? '隐藏数据库' : '显示数据库'}</Button>
        <Button size="small" variant={onlyErrors ? 'contained' : 'outlined'} onClick={() => setOnlyErrors((value) => !value)}>只看错误</Button>
      </Stack>
      <Divider />
      {rows.length === 0 ? <Typography color="text.secondary" sx={{ p: 2 }}>当前链路没有错误 Span。</Typography> : (
        <Box sx={{ maxHeight: 360, overflow: 'auto' }}>
          {rows.map((row) => {
            const category = spanCategory(row.span);
            const active = selected?.span_id === row.span.span_id;
            return (
              <Stack
                key={row.span.span_id}
                direction="row"
                alignItems="center"
                onClick={() => setSelectedID(row.span.span_id)}
                sx={{ minWidth: 760, px: 1.5, py: 0.8, cursor: 'pointer', bgcolor: active ? '#eef2ff' : 'transparent', borderBottom: '1px solid', borderColor: 'divider', '&:hover': { bgcolor: active ? '#eef2ff' : '#fafbfe' } }}
              >
                <Stack direction="row" spacing={0.8} alignItems="center" sx={{ width: '42%', minWidth: 320, pl: `${row.depth * 18}px`, overflow: 'hidden' }}>
                  <Chip size="small" label={category.label} sx={{ height: 20, bgcolor: category.color, color: '#fff', fontSize: 11 }} />
                  <Typography variant="body2" noWrap title={row.span.name}>{row.span.name}</Typography>
                  {row.orphaned && showDatabase && !onlyErrors && <Typography variant="caption" color="warning.main" title="父 Span 不在当前持久化数据中">断点</Typography>}
                </Stack>
                <Box sx={{ position: 'relative', width: '50%', height: 16, bgcolor: '#eef0f4', borderRadius: 8, overflow: 'hidden' }}>
                  <Box sx={{ position: 'absolute', left: `${row.offsetPercent}%`, width: `${row.widthPercent}%`, minWidth: 4, height: '100%', bgcolor: row.span.status_code.toLowerCase() === 'error' ? 'error.main' : category.color, borderRadius: 8 }} />
                </Box>
                <Typography variant="caption" color="text.secondary" sx={{ width: '8%', minWidth: 70, textAlign: 'right' }}>{formatDuration(row.span.duration_ms)}</Typography>
              </Stack>
            );
          })}
        </Box>
      )}
      {selected && (
        <>
          <Divider />
          <Box sx={{ p: 2 }}>
            <Stack direction={{ xs: 'column', md: 'row' }} spacing={2}>
              <Stack spacing={0.7} sx={{ minWidth: { md: 300 }, flex: 0.8 }}>
                <Stack direction="row" spacing={1} alignItems="center"><Chip size="small" label={spanCategory(selected).label} /><Typography fontWeight={700}>{selected.name}</Typography></Stack>
                <Typography variant="body2">状态：{selected.status_code}{selected.status_message ? ` · ${selected.status_message}` : ''}</Typography>
                <Typography variant="body2">耗时：{formatDuration(selected.duration_ms)}</Typography>
                <Typography variant="body2">Span ID：{selected.span_id}</Typography>
                <Typography variant="body2">Parent Span ID：{selected.parent_span_id || '-'}</Typography>
                <Typography variant="body2">服务：{selected.service_name || '-'}</Typography>
                <Typography variant="caption" color="text.secondary">{new Date(selected.started_at).toLocaleString('zh-CN')} → {new Date(selected.ended_at).toLocaleString('zh-CN')}</Typography>
              </Stack>
              <Stack spacing={1} sx={{ minWidth: 0, flex: 1.2 }}>
                <Typography variant="subtitle2">安全属性</Typography>
                <JsonBlock value={selected.attributes} />
                {selected.events.length > 0 && <><Typography variant="subtitle2">事件</Typography><JsonBlock value={selected.events} /></>}
              </Stack>
            </Stack>
          </Box>
        </>
      )}
    </Paper>
  );
}
