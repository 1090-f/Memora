import { useEffect, useReducer, useState } from 'react';
import ArrowBackOutlined from '@mui/icons-material/ArrowBackOutlined';
import CheckCircleOutlined from '@mui/icons-material/CheckCircleOutlined';
import ContentCopyOutlined from '@mui/icons-material/ContentCopyOutlined';
import RefreshOutlined from '@mui/icons-material/RefreshOutlined';

import {
  Alert,
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Chip,
  Divider,
  Grid,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TablePagination,
  TableRow,
  Typography,
} from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import ExpandMoreOutlined from '@mui/icons-material/ExpandMoreOutlined';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { listKnowledgeBases } from '@/features/knowledge-base/api';
import { listConversations } from '@/features/conversation/api';
import { streamAgentEvents } from '@/features/conversation/events';
import { getAgentRun, getAgentRunToolCalls, listAgentRuns } from '../api';
import { initialAgentRunState, reduceAgentEvent } from '../eventReducer';
import type { AgentRun, AgentRunViewState, AgentToolCall } from '../types';

const statusLabel: Record<string, string> = {
  queued: '排队中', running: '运行中', completed: '已完成', failed: '失败', cancelled: '已取消',
};

const selectedKnowledgeBaseStorageKey = 'agent-runs:selected-knowledge-base';

function storedKnowledgeBaseId() {
  try {
    return window.localStorage.getItem(selectedKnowledgeBaseStorageKey) || '';
  } catch {
    return '';
  }
}

function rememberKnowledgeBaseId(knowledgeBaseId: string) {
  try {
    if (knowledgeBaseId) window.localStorage.setItem(selectedKnowledgeBaseStorageKey, knowledgeBaseId);
    else window.localStorage.removeItem(selectedKnowledgeBaseStorageKey);
  } catch {
    // 隐私模式或存储被禁用时仍可使用当前页面内的选择。
  }
}

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString('zh-CN') : '-';
}

function StatusChip({ status }: { status: string }) {
  const color = status === 'completed' ? 'success' : status === 'failed' ? 'error' : status === 'running' ? 'info' : 'default';
  return <Chip size="small" color={color} label={statusLabel[status] || status} />;
}

function formatDuration(value?: number | null) {
  if (value == null) return '-';
  return value >= 1000 ? `${(value / 1000).toFixed(2)} s` : `${value} ms`;
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <Paper variant="outlined" sx={{ p: 2, height: '100%', bgcolor: '#fbfcff' }}>
      <Typography variant="caption" color="text.secondary">{label}</Typography>
      <Typography variant="h6" fontWeight={700} sx={{ mt: 0.5 }}>{value}</Typography>
    </Paper>
  );
}

function toolStatusLabel(status: AgentToolCall['status']) {
  return ({ running: '运行中', succeeded: '已完成', failed: '失败', timeout: '超时', cancelled: '已取消' }[status] || status);
}

function truncate(value: string, length = 280) {
  const normalized = value.replace(/\\n/g, '\n').trim();
  return normalized.length > length ? `${normalized.slice(0, length)}…` : normalized;
}

function parsePayload(value?: string) {
  if (!value) return null;
  try {
    return JSON.parse(value) as unknown;
  } catch {
    return value;
  }
}

function outputSummary(value?: string) {
  const payload = parsePayload(value);
  if (!payload) return '-';
  if (typeof payload === 'string') return truncate(payload);
  if (typeof payload !== 'object' || Array.isArray(payload)) return truncate(JSON.stringify(payload));

  const record = payload as Record<string, unknown>;
  const text = [record.text, record.summary, record.message, record.content]
    .find((item): item is string => typeof item === 'string' && item.trim().length > 0);
  const items = Array.isArray(record.items) ? record.items : undefined;
  const parts = [text ? truncate(text) : '', items ? `召回 ${items.length} 条结果` : ''].filter(Boolean);
  return parts.join(' · ') || truncate(JSON.stringify(record));
}

function inputSummary(value?: string) {
  const payload = parsePayload(value);
  if (!payload) return '-';
  if (typeof payload === 'string') return truncate(payload, 180);
  if (typeof payload !== 'object' || Array.isArray(payload)) return truncate(JSON.stringify(payload), 180);
  return Object.entries(payload as Record<string, unknown>)
    .slice(0, 4)
    .map(([key, item]) => `${key}: ${typeof item === 'string' ? item : JSON.stringify(item)}`)
    .join(' · ');
}

function RunTimeline({ run, toolCalls, liveState }: { run: AgentRun; toolCalls: AgentToolCall[]; liveState: AgentRunViewState }) {
  const liveSteps = liveState.plan?.steps.map((step) => ({ title: step.title, detail: step.status, duration: undefined, status: step.status }))
    ?? liveState.rounds.map((round) => ({ title: round.action_summary || `执行轮次 ${round.round_no}`, detail: round.status, duration: undefined, status: round.status }));
  const liveTools = liveState.tools.map((tool, index) => ({
    title: tool.tool_name,
    detail: tool.input_summary || tool.output_summary || '',
    duration: undefined,
    status: tool.status === 'completed' ? 'succeeded' : tool.status,
    call: toolCalls[index],
  }));
  const steps = [
    ...(liveState.router || run.execution_mode ? [{ title: '分析问题并确定执行模式', detail: liveState.router?.reason_summary || run.router_reason_summary || run.router_reason || `${run.execution_mode} 模式`, duration: undefined, status: 'completed' as const }] : []),
    ...(liveSteps.length > 0 ? liveSteps : []),
    ...(liveTools.length > 0 ? liveTools : toolCalls.map((call) => ({
      title: call.tool_name,
      detail: call.input_summary || call.output_summary || '',
      duration: call.duration_ms,
      status: call.status,
      call,
    }))),
    ...(run.final_result ? [{ title: '生成最终回答', detail: '回答生成完成', duration: undefined, status: 'completed' as const }] : []),
  ];

  if (steps.length === 0) return <Typography color="text.secondary">暂无执行步骤。</Typography>;

  return (
    <Stack spacing={0}>
      {steps.map((step, index) => {
        const call = 'call' in step ? step.call : undefined;
        const completed = step.status === 'completed' || step.status === 'succeeded';
        return (
          <Stack key={`${step.title}-${index}`} direction="row" spacing={1.5} sx={{ position: 'relative', pb: index === steps.length - 1 ? 0 : 2 }}>
            {index < steps.length - 1 && <Box sx={{ position: 'absolute', left: 10, top: 22, bottom: 0, borderLeft: '1px solid', borderColor: 'divider' }} />}
            <Box sx={{ zIndex: 1, width: 22, height: 22, borderRadius: '50%', display: 'grid', placeItems: 'center', bgcolor: completed ? 'success.main' : 'background.paper', border: '1px solid', borderColor: completed ? 'success.main' : 'primary.main', color: '#fff' }}>
              {completed ? <CheckCircleOutlined sx={{ fontSize: 15 }} /> : <Typography variant="caption" color="primary.main">{index + 1}</Typography>}
            </Box>
            <Box sx={{ minWidth: 0, flex: 1 }}>
              <Stack direction="row" alignItems="center" spacing={1}>
                <Typography fontWeight={650}>{step.title}</Typography>
                {call && <Chip size="small" variant="outlined" label={toolStatusLabel(call.status)} />}
                {step.duration != null && <Typography variant="caption" color="text.secondary" sx={{ ml: 'auto' }}>{formatDuration(step.duration)}</Typography>}
              </Stack>
              {step.detail && <Typography variant="body2" color="text.secondary" sx={{ mt: 0.4, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{call ? (call.input_summary ? inputSummary(step.detail) : outputSummary(step.detail)) : step.detail}</Typography>}
              {call && (
                <Accordion disableGutters variant="outlined" sx={{ mt: 1, bgcolor: '#fafbfe', '&:before': { display: 'none' } }}>
                  <AccordionSummary expandIcon={<ExpandMoreOutlined />} sx={{ minHeight: 40, '& .MuiAccordionSummary-content': { my: 1 } }}>
                    <Typography variant="body2">查看摘要</Typography>
                  </AccordionSummary>
                  <AccordionDetails sx={{ bgcolor: 'background.default', pt: 0 }}>
                    <Stack spacing={1.2}>
                      <Box>
                        <Typography variant="caption" color="text.secondary">输入</Typography>
                        <Typography variant="body2" sx={{ mt: 0.3, overflowWrap: 'anywhere' }}>{inputSummary(call.input_summary)}</Typography>
                      </Box>
                      <Box>
                        <Typography variant="caption" color="text.secondary">结果摘要</Typography>
                        <Typography variant="body2" sx={{ mt: 0.3, overflowWrap: 'anywhere' }}>{call.error_message || outputSummary(call.output_summary)}</Typography>
                      </Box>
                      <Box component="details" sx={{ '& summary': { cursor: 'pointer', color: 'text.secondary', fontSize: 12 } }}>
                        <Box component="summary">查看原始数据</Box>
                        <Box component="pre" sx={{ mt: 1, mb: 0, maxHeight: 220, overflow: 'auto', whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontSize: 11, color: 'text.secondary' }}>
                          {call.output_summary || '-'}
                        </Box>
                      </Box>
                    </Stack>
                  </AccordionDetails>
                </Accordion>
              )}
            </Box>
          </Stack>
        );
      })}
    </Stack>
  );
}

function RunDetail({ run }: { run: AgentRun }) {
  const [liveState, dispatch] = useReducer(reduceAgentEvent, initialAgentRunState);
  const toolCallsQuery = useQuery({
    queryKey: ['agent-run-tool-calls', run.id],
    queryFn: () => getAgentRunToolCalls(run.id),
  });
  useEffect(() => {
    dispatch({ type: 'HYDRATE_AGENT_RUN_STATE', run });
    const controller = new AbortController();
    void streamAgentEvents(`${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${run.id}/events`, {
      signal: controller.signal,
      afterSequence: 0,
      timeout: run.status === 'running' || run.status === 'queued' ? 0 : 30_000,
      onEvent: (event) => dispatch(event),
    }).catch(() => {
      // 运行详情仍可依赖 GET /runs 和 tool-calls 展示，事件回放失败不阻断页面。
    });
    return () => controller.abort();
  }, [run]);
  const copyRunId = () => void navigator.clipboard?.writeText(run.id);
  const toolCalls = toolCallsQuery.data ?? [];

  return (
    <Stack spacing={2}>
      <Stack direction="row" alignItems="center" spacing={1}>
        <Button component={Link} to="/runs" startIcon={<ArrowBackOutlined />}>返回列表</Button>
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1, overflowWrap: 'anywhere' }}>{run.query}</Typography>
        <StatusChip status={run.status} />
      </Stack>
      <Paper variant="outlined" sx={{ p: 3 }}>
        <Stack spacing={2}>
          <Stack direction="row" alignItems="center" spacing={1}>
            <Typography variant="body2" color="text.secondary" sx={{ overflowWrap: 'anywhere' }}>运行 ID：{run.id}</Typography>
            <Button size="small" startIcon={<ContentCopyOutlined />} onClick={copyRunId}>复制</Button>
          </Stack>
          <Divider />
          <Grid container spacing={1.5}>
            <Grid size={{ xs: 6, sm: 3 }}><Metric label="执行模式" value={run.execution_mode || '等待路由'} /></Grid>
            <Grid size={{ xs: 6, sm: 3 }}><Metric label="总耗时" value={formatDuration(run.duration_ms)} /></Grid>
            <Grid size={{ xs: 6, sm: 3 }}><Metric label="总 Token" value={`${run.total_tokens ?? 0}`} /></Grid>
            <Grid size={{ xs: 6, sm: 3 }}><Metric label="工具调用" value={`${Math.max(toolCalls.length, liveState.tools.length)} 次`} /></Grid>
          </Grid>
          <Typography variant="caption" color="text.secondary">
            Token 明细：输入 {run.input_tokens ?? 0} · 输出 {run.output_tokens ?? 0}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {formatDate(run.started_at)} → {formatDate(run.ended_at)}
          </Typography>
          {run.error_message && <Alert severity="error">{run.error_message}</Alert>}
          <Box>
            <Typography variant="h6" fontWeight={700} mb={1.5}>执行链路</Typography>
            {toolCallsQuery.error && <Alert severity="warning" sx={{ mb: 1.5 }}>工具调用详情加载失败，但仍可查看运行结果。</Alert>}
            <RunTimeline run={run} toolCalls={toolCalls} liveState={liveState} />
          </Box>
          {run.final_result && (
            <Paper variant="outlined" sx={{ p: 2, bgcolor: 'background.default' }}>
              <Typography fontWeight={700} mb={1.5}>最终回答</Typography>
              <Box className="markdown-body" sx={{ bgcolor: 'transparent' }}>
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{run.final_result}</ReactMarkdown>
              </Box>
            </Paper>
          )}
        </Stack>
      </Paper>
    </Stack>
  );
}

export function AgentRunListPage() {
  const { runId } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [selectedKbId, setSelectedKbId] = useState(storedKnowledgeBaseId);
  const [selectedConversationId, setSelectedConversationId] = useState('');
  const [selectedStatus, setSelectedStatus] = useState('');
  const [selectedExecutionMode, setSelectedExecutionMode] = useState('');
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(20);
  const knowledgeBasesQuery = useQuery({
    queryKey: ['agent-run-knowledge-bases'],
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 100 }),
  });
  useEffect(() => {
    const knowledgeBases = knowledgeBasesQuery.data?.items;
    if (!knowledgeBases) return;
    const nextKnowledgeBaseId = knowledgeBases.some((item) => item.id === selectedKbId)
      ? selectedKbId
      : knowledgeBases[0]?.id ?? '';
    if (nextKnowledgeBaseId === selectedKbId) return;
    setSelectedKbId(nextKnowledgeBaseId);
    setSelectedConversationId('');
    setPage(0);
  }, [knowledgeBasesQuery.data?.items, selectedKbId]);

  useEffect(() => {
    rememberKnowledgeBaseId(selectedKbId);
  }, [selectedKbId]);
  const conversationsQuery = useQuery({
    queryKey: ['agent-run-conversations', selectedKbId],
    queryFn: () => listConversations(selectedKbId, { page: 1, page_size: 100 }),
    enabled: Boolean(selectedKbId) && !runId,
  });
  const runsQuery = useQuery({
    queryKey: ['agent-runs', selectedKbId, selectedConversationId, selectedStatus, selectedExecutionMode, page, pageSize],
    queryFn: () => listAgentRuns({
      knowledge_base_id: selectedKbId,
      ...(selectedConversationId ? { conversation_id: selectedConversationId } : {}),
      ...(selectedStatus ? { status: selectedStatus } : {}),
      ...(selectedExecutionMode ? { execution_mode: selectedExecutionMode } : {}),
      page: page + 1,
      page_size: pageSize,
    }),
    enabled: Boolean(selectedKbId) && !runId,
  });
  const detailQuery = useQuery({
    queryKey: ['agent-run', runId],
    queryFn: () => getAgentRun(runId as string),
    enabled: Boolean(runId),
  });

  if (runId) {
    if (detailQuery.isPending) return <Typography>正在加载运行详情...</Typography>;
    if (detailQuery.error) return <Alert severity="error">运行详情加载失败，请刷新重试。</Alert>;
    if (detailQuery.data) return <RunDetail run={detailQuery.data} />;
  }

  return (
    <Stack spacing={3}>
      <Stack direction="row" alignItems="center">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>Agent 运行记录</Typography>
        <Button startIcon={<RefreshOutlined />} onClick={() => void queryClient.invalidateQueries({ queryKey: ['agent-runs', selectedKbId, selectedConversationId, selectedStatus, selectedExecutionMode] })}>刷新</Button>
      </Stack>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
        <FormControl size="small" sx={{ minWidth: 240 }}>
          <InputLabel id="run-kb-label">知识库</InputLabel>
          <Select labelId="run-kb-label" value={selectedKbId} label="知识库" onChange={(e) => {
            setSelectedKbId(e.target.value);
            setSelectedConversationId('');
            setPage(0);
          }}>
            {knowledgeBasesQuery.data?.items.map((kb) => <MenuItem key={kb.id} value={kb.id}>{kb.name}</MenuItem>)}
          </Select>
        </FormControl>
        <FormControl size="small" sx={{ minWidth: 280 }} disabled={!selectedKbId}>
          <InputLabel id="run-conversation-label">会话</InputLabel>
          <Select labelId="run-conversation-label" value={selectedConversationId} label="会话" onChange={(e) => {
            setSelectedConversationId(e.target.value);
            setPage(0);
          }}>
            <MenuItem value="">全部会话</MenuItem>
            {conversationsQuery.data?.items.map((conversation) => (
              <MenuItem key={conversation.id} value={conversation.id}>
                {conversation.title || `未命名会话（${conversation.id.slice(0, 8)}）`}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl size="small" sx={{ minWidth: 160 }} disabled={!selectedKbId}>
          <InputLabel id="run-status-label">状态</InputLabel>
          <Select labelId="run-status-label" value={selectedStatus} label="状态" onChange={(e) => {
            setSelectedStatus(e.target.value);
            setPage(0);
          }}>
            <MenuItem value="">全部状态</MenuItem>
            {Object.entries(statusLabel).map(([value, label]) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
          </Select>
        </FormControl>
        <FormControl size="small" sx={{ minWidth: 180 }} disabled={!selectedKbId}>
          <InputLabel id="run-mode-label">模式</InputLabel>
          <Select labelId="run-mode-label" value={selectedExecutionMode} label="模式" onChange={(e) => {
            setSelectedExecutionMode(e.target.value);
            setPage(0);
          }}>
            <MenuItem value="">全部模式</MenuItem>
            <MenuItem value="react">ReAct</MenuItem>
            <MenuItem value="plan_execute">Plan-Execute</MenuItem>
          </Select>
        </FormControl>
      </Stack>
      {!selectedKbId && <Typography color="text.secondary">请选择一个知识库查看运行记录。</Typography>}
      {runsQuery.error && <Alert severity="error">运行记录加载失败，请检查后端服务。</Alert>}
      <Paper variant="outlined">
        <Table>
          <TableHead><TableRow><TableCell>问题</TableCell><TableCell>状态</TableCell><TableCell>模式</TableCell><TableCell>耗时</TableCell><TableCell>创建时间</TableCell></TableRow></TableHead>
          <TableBody>
            {runsQuery.data?.items.map((run) => (
              <TableRow key={run.id} hover sx={{ cursor: 'pointer' }} onClick={() => navigate(`/runs/${run.id}`)}>
                <TableCell sx={{ maxWidth: 420, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{run.query}</TableCell>
                <TableCell><StatusChip status={run.status} /></TableCell>
                <TableCell>{run.execution_mode || '-'}</TableCell>
                <TableCell>{run.duration_ms == null ? '-' : `${run.duration_ms} ms`}</TableCell>
                <TableCell>{formatDate(run.created_at)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {runsQuery.data && (
          <TablePagination
            component="div"
            count={runsQuery.data.total}
            page={page}
            onPageChange={(_, newPage) => setPage(newPage)}
            rowsPerPage={pageSize}
            onRowsPerPageChange={(e) => { setPageSize(parseInt(e.target.value, 10)); setPage(0); }}
            rowsPerPageOptions={[10, 20, 50, 100]}
            labelRowsPerPage="每页条数:"
            labelDisplayedRows={({ page, count }) => `页数 ${page + 1}，共 ${count} 条`}
          />
        )}
        {runsQuery.data?.items.length === 0 && <Typography color="text.secondary" sx={{ p: 3 }}>暂无运行记录。</Typography>}
      </Paper>
    </Stack>
  );
}
