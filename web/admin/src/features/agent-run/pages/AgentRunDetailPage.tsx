import { Alert, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { getAgentRun } from '../api';
import { RunDetail } from './AgentRunListPage';

export function AgentRunDetailPage() {
  const { runId } = useParams();
  const detailQuery = useQuery({
    queryKey: ['agent-run', runId],
    queryFn: () => getAgentRun(runId as string),
    enabled: Boolean(runId),
  });

  if (!runId) return <Alert severity="error">运行 ID 缺失。</Alert>;
  if (detailQuery.isPending) return <Typography>正在加载运行详情...</Typography>;
  if (detailQuery.error) return <Alert severity="error">运行详情加载失败，请刷新重试。</Alert>;
  return detailQuery.data ? <RunDetail run={detailQuery.data} /> : null;
}
