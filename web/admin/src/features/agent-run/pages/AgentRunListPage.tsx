import { Button, Paper, Stack, Typography } from '@mui/material';
import { useParams } from 'react-router-dom';
import { UnavailableState } from '@/components/shared/UnavailableState';

export function AgentRunListPage() {
  const { runId } = useParams();
  return (
    <Stack spacing={3}>
      <Stack direction="row" alignItems="center">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>
          {runId ? 'Agent 运行详情' : 'Agent 运行记录'}
        </Typography>
        <Button disabled variant="outlined">重试失败任务</Button>
      </Stack>
      <UnavailableState
        title="Agent 运行后端待接入"
        description="Router、Plan、ReAct 轮次、工具调用和引用会按可观察摘要展示，不包含模型隐藏推理。"
        capability="agentRun"
      />
      <Paper variant="outlined" sx={{ p: 3 }}>
        <Typography color="text.secondary">{runId ? `运行 ID：${runId}` : '暂无可加载的运行记录。'}</Typography>
      </Paper>
    </Stack>
  );
}
