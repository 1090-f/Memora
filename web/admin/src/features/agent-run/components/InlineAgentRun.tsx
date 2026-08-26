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
import { timelineEntries } from '../timeline';
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
  completed: '成功',
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

// TimelineDot 渲染时间线上的序号圆点。
function TimelineDot({ status, index }: { status: string; index: number }) {
  const completed = status === 'completed' || status === 'succeeded' || status === 'cancelled';
  const active = status === 'running';
  return (
    <Box sx={{ width: 18, height: 18, flexShrink: 0, display: 'grid', placeItems: 'center', borderRadius: '50%', bgcolor: completed ? '#31a75b' : active ? '#586de8' : '#dce1e9', color: '#fff', fontSize: 10 }}>
      {completed ? <CheckCircleRounded sx={{ fontSize: 14 }} /> : index + 1}
    </Box>
  );
}

export function InlineAgentRun({ state, runId }: { state: AgentRunViewState; runId?: string }) {
  const [expanded, setExpanded] = useState(state.status === 'running' || state.status === 'queued');
  const [toolRecords, setToolRecords] = useState<AgentToolCall[] | null>(null);
  const [recordsLoading, setRecordsLoading] = useState(false);
  const [expandedToolKey, setExpandedToolKey] = useState<string | null>(null);
  const [expandedEntryKey, setExpandedEntryKey] = useState<string | null>(null);
  const running = state.status === 'running' || state.status === 'queued';
  const failed = state.status === 'failed';
  const entries = timelineEntries(state);

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
    if (!runId || running || state.timeline.some((entry) => entry.kind === 'tool')) return;
    void fetchToolRecords();
  }, [runId, running, state.timeline, fetchToolRecords]);

  // DB 工具调用记录与 timeline 中 tool 条目按时间顺序一一对应。
  const toolRecordBySeq = new Map<number, AgentToolCall | undefined>();
  {
    let toolIndex = 0;
    for (const entry of entries) {
      if (entry.kind === 'tool') {
        toolRecordBySeq.set(entry.sequence, toolRecords?.[toolIndex]);
        toolIndex += 1;
      }
    }
  }

  const handleEntryClick = (key: string) => {
    setExpandedEntryKey((prev) => (prev === key ? null : key));
  };

  const detailCount = entries.length;

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
            {state.error?.message || state.router?.reason_summary || (running ? '正在分析问题并调用所需工具' : `共 ${detailCount} 项执行记录`)}
          </Typography>
        </Box>
        <IconButton size="small" aria-label={expanded ? '收起运行详情' : '展开运行详情'} sx={{ color: '#7c8799' }}>
          <ExpandMoreRounded sx={{ fontSize: 20, transform: expanded ? 'rotate(180deg)' : 'none', transition: 'transform .2s' }} />
        </IconButton>
      </Stack>

      <Collapse in={expanded}>
        <Stack spacing={1.3} sx={{ borderTop: '1px solid #e5e9f0', px: 1.6, py: 1.4 }}>
          <Stack spacing={0.9}>
            <Typography sx={{ color: '#6d788a', fontSize: 11, fontWeight: 700 }}>执行链路</Typography>
            {entries.length > 0 ? (
              <Stack spacing={0.9}>
                {entries.map((entry, index) => {
                  const key = `${entry.sequence}-${index}`;
                  switch (entry.kind) {
                    case 'status':
                      return (
                        <Stack key={key} direction="row" spacing={1} alignItems="center" sx={{ pl: 0.5 }}>
                          <Box sx={{ width: 7, height: 7, flexShrink: 0, borderRadius: '50%', bgcolor: entry.status === 'failed' ? '#e05050' : entry.status === 'running' ? '#586de8' : '#c3ccda' }} />
                          <Typography sx={{ color: entry.status === 'failed' ? '#bd4a4a' : '#6d788a', fontSize: 11.5, fontWeight: entry.status === 'failed' ? 650 : 500 }}>{entry.title}</Typography>
                        </Stack>
                      );
                    case 'router':
                      return (
                        <Stack key={key} spacing={0.2} sx={{ pl: 0.5 }}>
                          <Stack direction="row" alignItems="center" spacing={0.5} onClick={() => handleEntryClick(key)} sx={{ cursor: 'pointer' }}>
                            <Typography sx={{ color: '#4358d0', fontSize: 12.5, fontWeight: 650 }}>
                              {entry.execution_mode === 'plan_execute' ? '计划执行模式' : 'ReAct 协作模式'}
                            </Typography>
                            <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>
                              {expandedEntryKey === key ? '▲' : '▼'}
                            </Typography>
                          </Stack>
                          {entry.reason_summary && <Typography sx={{ color: '#8c96a7', fontSize: 11 }}>{entry.reason_summary}</Typography>}
                          {expandedEntryKey === key && (
                            <Stack spacing={0.5} sx={{ ml: 1.5, mt: 0.5 }}>
                              {entry.input_summary && (
                                <DetailBlock label="输入" value={entry.input_summary} empty="" />
                              )}
                              {(entry.confidence !== undefined || entry.fallback_used !== undefined) && (
                                <Stack direction="row" spacing={1.5} sx={{ mt: 0.5 }}>
                                  {entry.confidence !== undefined && (
                                    <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>置信度: {entry.confidence}</Typography>
                                  )}
                                  {entry.fallback_used !== undefined && (
                                    <Typography sx={{ color: entry.fallback_used ? '#c98a2e' : '#2ca457', fontSize: 10.5 }}>
                                      {entry.fallback_used ? '已使用兜底策略' : '未使用兜底策略'}
                                    </Typography>
                                  )}
                                </Stack>
                              )}
                            </Stack>
                          )}
                        </Stack>
                      );
                    case 'plan_created':
                      return (
                        <Stack key={key} spacing={0.2} sx={{ pl: 0.5 }}>
                          <Stack direction="row" alignItems="center" spacing={0.5} onClick={() => handleEntryClick(key)} sx={{ cursor: 'pointer' }}>
                            <TimelineDot status="completed" index={index} />
                            <Typography sx={{ color: '#4e5a6f', fontSize: 12.5, flex: 1 }}>
                              {entry.replanned ? `重新规划执行计划（v${entry.version} · ${entry.step_count} 步）` : `制定执行计划（v${entry.version} · ${entry.step_count} 步）`}
                            </Typography>
                            <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>
                              {expandedEntryKey === key ? '▲' : '▼'}
                            </Typography>
                          </Stack>
                          {expandedEntryKey === key && (
                            <Stack spacing={0.5} sx={{ ml: 1.5, mt: 0.5 }}>
                              {entry.input_summary && <DetailBlock label="输入" value={entry.input_summary} empty="" />}
                              {entry.steps_detail && entry.steps_detail.length > 0 && (
                                <Stack spacing={0.3}>
                                  <Typography sx={{ color: '#6d788a', fontSize: 11, fontWeight: 650 }}>步骤详情</Typography>
                                  {entry.steps_detail.map((step: any, si: number) => (
                                    <Stack key={si} spacing={0.2} sx={{ pl: 1 }}>
                                      <Typography sx={{ color: '#4e5a6f', fontSize: 11.5, fontWeight: 550 }}>
                                        步骤 {step.step_no}: {step.title}
                                      </Typography>
                                      {step.description && <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>{step.description}</Typography>}
                                      {step.recommended_tool && <Typography sx={{ color: '#5b6ee1', fontSize: 10.5 }}>推荐工具: {step.recommended_tool}</Typography>}
                                      {step.depends_on && step.depends_on.length > 0 && <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>依赖: {step.depends_on.join(', ')}</Typography>}
                                    </Stack>
                                  ))}
                                </Stack>
                              )}
                            </Stack>
                          )}
                        </Stack>
                      );
                    case 'plan_step':
                    case 'round': {
                      const isRound = entry.kind === 'round';
                      const label = isRound ? (entry.action_summary || `执行轮次 ${entry.round_no}`) : entry.title;
                      const hasDetail = (entry.input_summary || entry.output_summary ||
                        (isRound && entry.model_decision) ||
                        entry.duration_ms || entry.token_usage);
                      return (
                        <Stack key={key} spacing={0.2} sx={{ pl: 0.5 }}>
                          <Stack direction="row" alignItems="center" spacing={0.5} onClick={() => handleEntryClick(key)} sx={{ cursor: hasDetail ? 'pointer' : 'default' }}>
                            <TimelineDot status={entry.status} index={index} />
                            <Typography sx={{ color: entry.status === 'running' ? '#4358d0' : '#4e5a6f', fontSize: 12.5, flex: 1 }}>
                              {label}
                            </Typography>
                            <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>
                              {entry.status === 'completed' ? '已完成' : entry.status === 'failed' ? '失败' : entry.status === 'running' ? '进行中' : '等待中'}
                            </Typography>
                            {hasDetail && <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>{expandedEntryKey === key ? '▲' : '▼'}</Typography>}
                          </Stack>
                          {expandedEntryKey === key && hasDetail && (
                            <Stack spacing={0.5} sx={{ ml: 1.5, mt: 0.5 }}>
                              {entry.input_summary && <DetailBlock label="输入" value={entry.input_summary} empty="" />}
                              {isRound && entry.model_decision && <DetailBlock label="模型决策" value={entry.model_decision} empty="" />}
                              {entry.output_summary && <DetailBlock label="输出" value={entry.output_summary} empty="" />}
                              {entry.duration_ms && (
                                <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>耗时 {formatDuration(entry.duration_ms)}</Typography>
                              )}
                              {entry.token_usage && (
                                <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>
                                  Tokens: 输入 {entry.token_usage.input_tokens} · 输出 {entry.token_usage.output_tokens} · 共 {entry.token_usage.total_tokens}
                                </Typography>
                              )}
                            </Stack>
                          )}
                        </Stack>
                      );
                    }
                    case 'tool': {
                      const record = toolRecordBySeq.get(entry.sequence);
                      const failedTool = record?.status === 'failed' || entry.status === 'failed';
                      const toolStatus = record ? record.status : entry.status;
                      const toolStatusLabel = record ? toolStatusText[record.status] || record.status : entry.status === 'completed' ? '✓ 已完成' : entry.status === 'failed' ? '✕ 失败' : '◌ 运行中';
                      const handleToolRecordClick = (key: string) => {
                        setExpandedToolKey((prev) => (prev === key ? null : key));
                      };
                      return (
                        <Stack key={key} spacing={0.6}>
                          <Stack direction="row" spacing={1} alignItems="center">
                            <Box sx={{ width: 18, flexShrink: 0, display: 'grid', placeItems: 'center' }}>
                              <BuildOutlined sx={{ color: failedTool ? '#e05050' : '#5b6ee1', fontSize: 14 }} />
                            </Box>
                            <Chip
                              size="small"
                              label={entry.tool_name}
                              clickable
                              variant="outlined"
                              onClick={() => handleToolRecordClick(entry.tool_call_id)}
                              sx={{
                                bgcolor: expandedToolKey === entry.tool_call_id ? '#e8edff' : '#fff',
                                borderColor: expandedToolKey === entry.tool_call_id ? '#aebdff' : '#dde3ec',
                                color: expandedToolKey === entry.tool_call_id ? '#394ec8' : failedTool ? '#bd4a4a' : '#536078',
                                fontSize: 11,
                                cursor: 'pointer',
                                '& .MuiChip-icon': { color: failedTool ? '#bd4a4a' : '#5b6ee1' },
                                '&:hover': { bgcolor: '#f2f5ff' },
                              }}
                            />
                            <Typography sx={{ color: toolStatus === 'failed' ? '#bd4a4a' : toolStatus === 'completed' || toolStatus === 'succeeded' ? '#2ca457' : toolStatus === 'running' ? '#5670ec' : '#8c96a7', fontSize: 10.5 }}>
                              {toolStatusLabel}
                            </Typography>
                          </Stack>
                          {expandedToolKey === entry.tool_call_id && (
                            <Stack spacing={1} sx={{ border: '1px solid #e1e6ef', borderRadius: 1.5, bgcolor: '#fbfcfe', p: 1.4, ml: 3 }}>
                              <Stack direction="row" alignItems="center" spacing={1}>
                                <Typography sx={{ color: '#33405a', fontSize: 12, fontWeight: 650 }}>{entry.tool_name}</Typography>
                                {record && (
                                  <Chip size="small" label={toolStatusText[record.status] || record.status} sx={{ height: 20, bgcolor: record.status === 'failed' ? '#fff0f0' : '#eaf9f1', color: record.status === 'failed' ? '#bd4a4a' : '#278b4d', fontSize: 10, fontWeight: 650 }} />
                                )}
                                {typeof record?.duration_ms === 'number' && (
                                  <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>耗时 {formatDuration(record.duration_ms)}</Typography>
                                )}
                                {record?.is_truncated && (
                                  <Typography sx={{ color: '#c98a2e', fontSize: 10.5 }}>结果已截断</Typography>
                                )}
                              </Stack>
                              <DetailBlock label="输入" value={record?.input_summary || entry.input_summary} empty="无输入参数记录" />
                              <DetailBlock label="输出" value={record?.output_summary || entry.output_summary} empty="无输出结果记录" />
                              {(record?.error_message || entry.error_message) && (
                                <DetailBlock label="错误信息" value={record?.error_message || entry.error_message} empty="" />
                              )}
                            </Stack>
                          )}
                        </Stack>
                      );
                    }
                    case 'citation': {
                      const name = citationName(entry.citation, index);
                      return (
                        <Stack key={key} direction="row" spacing={1} alignItems="center" sx={{ pl: 0.5 }}>
                          <DescriptionOutlined sx={{ color: '#6574dc', fontSize: 14 }} />
                          <Chip size="small" icon={<DescriptionOutlined />} label={name} sx={{ maxWidth: 220, bgcolor: '#eef2fa', color: '#536078', fontSize: 11, '& .MuiChip-icon': { color: '#6574dc' } }} />
                        </Stack>
                      );
                    }
                    case 'answer':
                      return (
                        <Stack key={key} direction="row" spacing={1} alignItems="center">
                          <TimelineDot status="completed" index={index} />
                          <Typography sx={{ color: '#4e5a6f', fontSize: 12.5, flex: 1 }}>生成最终回答</Typography>
                        </Stack>
                      );
                    default:
                      return null;
                  }
                })}
              </Stack>
            ) : (
              <Typography sx={{ color: '#8c96a7', fontSize: 11.5 }}>暂无执行记录。</Typography>
            )}
          </Stack>

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