import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import BuildOutlined from '@mui/icons-material/BuildOutlined';
import CheckCircleRounded from '@mui/icons-material/CheckCircleRounded';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import ErrorOutlineRounded from '@mui/icons-material/ErrorOutlineRounded';
import ExpandMoreRounded from '@mui/icons-material/ExpandMoreRounded';
import SyncRounded from '@mui/icons-material/SyncRounded';
import { Box, Chip, Collapse, IconButton, Stack, Typography } from '@mui/material';
import { useCallback, useEffect, useState } from 'react';
import { getAgentRunToolCalls } from '../api';
import type { AgentRunViewState, AgentToolCall } from '../types';

const statusText = {
  idle: '等待中',
  queued: '正在排队',
  running: '正在运行',
  completed: '运行完成',
  failed: '运行失败',
  cancelled: '已停止',
} as const;

const toolStatusText: Record<string, string> = {
  running: '进行中',
  succeeded: '成功',
  failed: '失败',
  timeout: '超时',
  cancelled: '已取消',
};

function citationName(citation: Record<string, unknown>, index: number) {
  const value = citation.document_title || citation.title || citation.url;
  return typeof value === 'string' && value ? value : `引用 ${index + 1}`;
}

function formatDuration(durationMs?: number | null) {
  if (typeof durationMs !== 'number') return '';
  if (durationMs < 1000) return `${durationMs} ms`;
  return `${(durationMs / 1000).toFixed(2)} s`;
}

function DetailBlock({ label, value, empty }: { label: string; value?: string; empty: string }) {
  return (
    <Stack spacing={0.4}>
      <Typography sx={{ color: '#6d788a', fontSize: 11, fontWeight: 700 }}>{label}</Typography>
      {value ? (
        <Box component="pre" sx={{ m: 0, p: 1.2, borderRadius: 1.2, bgcolor: '#f1f4f9', color: '#3c4759', fontSize: 12, lineHeight: 1.6, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontFamily: 'Consolas, "SFMono-Regular", monospace', maxHeight: 240, overflow: 'auto' }}>
          {value}
        </Box>
      ) : (
        <Typography sx={{ color: '#a5adbc', fontSize: 11.5, fontStyle: 'italic' }}>{empty}</Typography>
      )}
    </Stack>
  );
}

export function InlineAgentRun({ state, runId }: { state: AgentRunViewState; runId?: string }) {
  const [expanded, setExpanded] = useState(state.status === 'running' || state.status === 'queued');
  const [toolRecords, setToolRecords] = useState<AgentToolCall[] | null>(null);
  const [recordsLoading, setRecordsLoading] = useState(false);
  const [expandedToolKey, setExpandedToolKey] = useState<string | null>(null);
  const running = state.status === 'running' || state.status === 'queued';
  const failed = state.status === 'failed';
  const steps = state.plan?.steps ?? state.rounds.map((round) => ({
    step_no: round.round_no,
    title: round.action_summary || `执行轮次 ${round.round_no}`,
    status: round.status,
  }));

  const fetchToolRecords = useCallback(async () => {
    if (!runId || toolRecords !== null || recordsLoading) return;
    setRecordsLoading(true);
    try {
      const records = await getAgentRunToolCalls(runId);
      setToolRecords(Array.isArray(records) ? records : []);
    } catch {
      setToolRecords([]);
    } finally {
      setRecordsLoading(false);
    }
  }, [runId, toolRecords, recordsLoading]);

  // 当链路中没有实时工具事件（如 Plan-Execute 模式）时，从服务端补齐工具调用记录。
  useEffect(() => {
    if (!runId || running || state.tools.length > 0) return;
    void fetchToolRecords();
  }, [runId, running, state.tools.length, fetchToolRecords]);

  // 归一化要展示的工具条目：优先使用实时事件，其次使用服务端记录。
  const tools = state.tools.length > 0
    ? state.tools.map((tool, index) => ({
        key: tool.tool_call_id || `live-${index}`,
        name: tool.tool_name,
        record: toolRecords && toolRecords.length > index ? toolRecords[index] : undefined,
        fallbackInput: tool.input_summary,
        fallbackOutput: tool.output_summary,
      }))
    : (toolRecords ?? []).map((record, index) => ({
        key: record.id || `${record.tool_name}-${index}`,
        name: record.tool_name,
        record,
        fallbackInput: undefined,
        fallbackOutput: undefined,
      }));

  const handleToolClick = (key: string) => {
    if (state.tools.length > 0) void fetchToolRecords();
    setExpandedToolKey((prev) => (prev === key ? null : key));
  };

  const detailCount = steps.length + tools.length + state.citations.length;

  if (state.status === 'idle') return null;

  return (
    <Box
      component="section"
      aria-label="Agent 运行过程"
      sx={{
        width: 'calc(100% - 46px)',
        ml: '46px',
        border: '1px solid',
        borderColor: failed ? '#f1c8c8' : '#e0e6ef',
        borderRadius: 2.5,
        bgcolor: '#f8fafc',
        overflow: 'hidden',
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        spacing={1.1}
        onClick={() => setExpanded((value) => !value)}
        sx={{ px: 1.5, py: 1.2, cursor: 'pointer', userSelect: 'none' }}
      >
        <Box sx={{ width: 28, height: 28, display: 'grid', placeItems: 'center', borderRadius: 1.4, color: '#5268e8', bgcolor: '#e9edff' }}>
          <AutoAwesomeOutlined sx={{ fontSize: 17 }} />
        </Box>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Stack direction="row" alignItems="center" spacing={0.8}>
            <Typography sx={{ color: '#26324d', fontSize: 13, fontWeight: 700 }}>Agent 运行</Typography>
            <Chip
              size="small"
              icon={running ? <SyncRounded className="animate-spin" /> : failed ? <ErrorOutlineRounded /> : <CheckCircleRounded />}
              label={statusText[state.status]}
              sx={{
                height: 22,
                bgcolor: failed ? '#fff0f0' : running ? '#edf1ff' : '#eaf8ef',
                color: failed ? '#bd4a4a' : running ? '#4e63dd' : '#278b4d',
                fontSize: 10.5,
                fontWeight: 650,
                '& .MuiChip-icon': { color: 'inherit', fontSize: 14 },
              }}
            />
          </Stack>
          <Typography noWrap sx={{ color: '#7b8799', fontSize: 11.5, mt: 0.15 }}>
            {state.error?.message || state.router?.reason_summary || (running ? '正在分析问题并调用所需工具' : `${detailCount} 项运行记录`)}
          </Typography>
        </Box>
        <IconButton size="small" aria-label={expanded ? '收起运行详情' : '展开运行详情'} sx={{ color: '#7c8799' }}>
          <ExpandMoreRounded sx={{ fontSize: 20, transform: expanded ? 'rotate(180deg)' : 'none', transition: 'transform .2s' }} />
        </IconButton>
      </Stack>

      <Collapse in={expanded}>
        <Stack spacing={1.3} sx={{ borderTop: '1px solid #e5e9f0', px: 1.6, py: 1.4 }}>
          {steps.length > 0 && (
            <Stack spacing={0.8}>
              <Typography sx={{ color: '#6d788a', fontSize: 11, fontWeight: 700 }}>执行步骤</Typography>
              {steps.map((step, index) => {
                const completed = step.status === 'completed' || step.status === 'succeeded';
                const active = step.status === 'running';
                return (
                  <Stack key={`${step.step_no}-${index}`} direction="row" spacing={1} alignItems="center">
                    <Box sx={{ width: 18, height: 18, flexShrink: 0, display: 'grid', placeItems: 'center', borderRadius: '50%', bgcolor: completed ? '#31a75b' : active ? '#586de8' : '#dce1e9', color: '#fff', fontSize: 10 }}>
                      {completed ? <CheckCircleRounded sx={{ fontSize: 14 }} /> : index + 1}
                    </Box>
                    <Typography sx={{ color: active ? '#4358d0' : '#4e5a6f', fontSize: 12.5, flex: 1 }}>{step.title}</Typography>
                    <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>{completed ? '已完成' : active ? '进行中' : '等待中'}</Typography>
                  </Stack>
                );
              })}
            </Stack>
          )}

          {tools.length > 0 && (
            <Stack spacing={0.7}>
              <Typography sx={{ color: '#6d788a', fontSize: 11, fontWeight: 700 }}>工具调用</Typography>
              <Stack direction="row" useFlexGap flexWrap="wrap" gap={0.7}>
                {tools.map((tool) => {
                  const failedTool = tool.record?.status === 'failed';
                  return (
                    <Chip
                      key={tool.key}
                      size="small"
                      icon={<BuildOutlined />}
                      label={tool.name}
                      clickable
                      variant="outlined"
                      onClick={() => handleToolClick(tool.key)}
                      sx={{
                        bgcolor: expandedToolKey === tool.key ? '#e8edff' : '#fff',
                        borderColor: expandedToolKey === tool.key ? '#aebdff' : '#dde3ec',
                        color: expandedToolKey === tool.key ? '#394ec8' : failedTool ? '#bd4a4a' : '#536078',
                        fontSize: 11,
                        cursor: 'pointer',
                        '& .MuiChip-icon': { color: failedTool ? '#bd4a4a' : '#5b6ee1' },
                        '&:hover': { bgcolor: '#f2f5ff' },
                      }}
                    />
                  );
                })}
              </Stack>
              {recordsLoading && !toolRecords && (
                <Typography sx={{ color: '#9aa4b5', fontSize: 10.5 }}>正在加载工具调用记录…</Typography>
              )}
              {expandedToolKey && (() => {
                const tool = tools.find((item) => item.key === expandedToolKey);
                if (!tool) return null;
                const record = tool.record;
                const statusLabel = record ? toolStatusText[record.status] : undefined;
                return (
                  <Stack spacing={1} sx={{ border: '1px solid #e1e6ef', borderRadius: 1.5, bgcolor: '#fbfcfe', p: 1.4 }}>
                    <Stack direction="row" alignItems="center" spacing={1}>
                      <Typography sx={{ color: '#33405a', fontSize: 12, fontWeight: 650 }}>{tool.name}</Typography>
                      {statusLabel && (
                        <Chip size="small" label={statusLabel} sx={{ height: 20, bgcolor: record?.status === 'failed' ? '#fff0f0' : '#eaf9f1', color: record?.status === 'failed' ? '#bd4a4a' : '#278b4d', fontSize: 10, fontWeight: 650 }} />
                      )}
                      {typeof record?.duration_ms === 'number' && (
                        <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>耗时 {formatDuration(record.duration_ms)}</Typography>
                      )}
                      {record?.is_truncated && (
                        <Typography sx={{ color: '#c98a2e', fontSize: 10.5 }}>结果已截断</Typography>
                      )}
                    </Stack>
                    <DetailBlock label="输入" value={record?.input_summary || tool.fallbackInput} empty="无输入参数记录" />
                    <DetailBlock label="输出" value={record?.output_summary || tool.fallbackOutput} empty="无输出结果记录" />
                    {record?.error_message && (
                      <DetailBlock label="错误信息" value={record.error_message} empty="" />
                    )}
                  </Stack>
                );
              })()}
            </Stack>
          )}

          {state.citations.length > 0 && (
            <Stack spacing={0.7}>
              <Typography sx={{ color: '#6d788a', fontSize: 11, fontWeight: 700 }}>引用文档</Typography>
              <Stack direction="row" useFlexGap flexWrap="wrap" gap={0.7}>
                {state.citations.slice(0, 5).map((citation, index) => (
                  <Chip key={`${citationName(citation, index)}-${index}`} size="small" icon={<DescriptionOutlined />} label={citationName(citation, index)} sx={{ maxWidth: 220, bgcolor: '#eef2fa', color: '#536078', fontSize: 11, '& .MuiChip-icon': { color: '#6574dc' } }} />
                ))}
              </Stack>
            </Stack>
          )}

          {state.usage && (
            <Typography sx={{ color: '#8a94a5', fontSize: 10.5 }}>
              输入 {state.usage.input_tokens} · 输出 {state.usage.output_tokens} · 共 {state.usage.total_tokens} Tokens
            </Typography>
          )}
        </Stack>
      </Collapse>
    </Box>
  );
}