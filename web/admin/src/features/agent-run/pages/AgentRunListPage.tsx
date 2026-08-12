import { useState } from 'react';
import ArrowBackOutlined from '@mui/icons-material/ArrowBackOutlined';
import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import ReplayOutlined from '@mui/icons-material/ReplayOutlined';
import {
  Alert,
  Button,
  Chip,
  Divider,
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
import { Link, useNavigate, useParams } from 'react-router-dom';
import { listKnowledgeBases } from '@/features/knowledge-base/api';
import { getAgentRun, listAgentRuns, retryAgentRun } from '../api';
import type { AgentRun } from '../types';

const statusLabel: Record<string, string> = {
  queued: '排队中', running: '运行中', completed: '已完成', failed: '失败', cancelled: '已取消',
};

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleString('zh-CN') : '-';
}

function StatusChip({ status }: { status: string }) {
  const color = status === 'completed' ? 'success' : status === 'failed' ? 'error' : status === 'running' ? 'info' : 'default';
  return <Chip size="small" color={color} label={statusLabel[status] || status} />;
}

function RunDetail({ run, onRetry }: { run: AgentRun; onRetry: () => void }) {
  return (
    <Stack spacing={2}>
      <Stack direction="row" alignItems="center" spacing={1}>
        <Button component={Link} to="/runs" startIcon={<ArrowBackOutlined />}>返回列表</Button>
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>运行详情</Typography>
        <StatusChip status={run.status} />
        {run.status === 'failed' && <Button variant="outlined" startIcon={<ReplayOutlined />} onClick={onRetry}>重试</Button>}
      </Stack>
      <Paper variant="outlined" sx={{ p: 3 }}>
        <Stack spacing={2}>
          <Typography variant="body2" color="text.secondary">运行 ID：{run.id}</Typography>
          <Typography variant="h6" sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{run.query}</Typography>
          <Divider />
          <Typography>执行模式：{run.execution_mode || '等待路由结果'}</Typography>
          <Typography>开始时间：{formatDate(run.started_at)}</Typography>
          <Typography>结束时间：{formatDate(run.ended_at)}</Typography>
          <Typography>耗时：{run.duration_ms == null ? '-' : `${run.duration_ms} ms`}</Typography>
          <Typography>Token：{run.total_tokens ?? 0}</Typography>
          {run.error_message && <Alert severity="error">{run.error_message}</Alert>}
          {run.final_result && (
            <Paper variant="outlined" sx={{ p: 2, bgcolor: 'background.default' }}>
              <Typography fontWeight={700} mb={1}>最终回答</Typography>
              <Typography sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{run.final_result}</Typography>
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
  const [selectedKbId, setSelectedKbId] = useState('');
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(20);
  const knowledgeBasesQuery = useQuery({
    queryKey: ['agent-run-knowledge-bases'],
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 100 }),
  });
  const runsQuery = useQuery({
    queryKey: ['agent-runs', selectedKbId, page, pageSize],
    queryFn: () => listAgentRuns({ knowledge_base_id: selectedKbId, page: page + 1, page_size: pageSize }),
    enabled: Boolean(selectedKbId) && !runId,
  });
  const detailQuery = useQuery({
    queryKey: ['agent-run', runId],
    queryFn: () => getAgentRun(runId as string),
    enabled: Boolean(runId),
  });

  const retry = async (id: string) => {
    const response = await retryAgentRun(id);
    navigate(`/runs/${response.new_run_id}`);
  };

  if (runId) {
    if (detailQuery.isPending) return <Typography>正在加载运行详情...</Typography>;
    if (detailQuery.error) return <Alert severity="error">运行详情加载失败，请刷新重试。</Alert>;
    if (detailQuery.data) return <RunDetail run={detailQuery.data} onRetry={() => void retry(detailQuery.data.id)} />;
  }

  return (
    <Stack spacing={3}>
      <Stack direction="row" alignItems="center">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>Agent 运行记录</Typography>
        <Button startIcon={<RefreshOutlined />} onClick={() => void queryClient.invalidateQueries({ queryKey: ['agent-runs', selectedKbId] })}>刷新</Button>
      </Stack>
      <FormControl size="small" sx={{ maxWidth: 360 }}>
        <InputLabel id="run-kb-label">知识库</InputLabel>
        <Select labelId="run-kb-label" value={selectedKbId} label="知识库" onChange={(e) => { setSelectedKbId(e.target.value); setPage(0); }}>
          {knowledgeBasesQuery.data?.items.map((kb) => <MenuItem key={kb.id} value={kb.id}>{kb.name}</MenuItem>)}
        </Select>
      </FormControl>
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
