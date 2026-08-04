import { Button, MenuItem, Paper, Stack, TextField, Typography } from '@mui/material';
import { useParams } from 'react-router-dom';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { UnavailableState } from '@/components/shared/UnavailableState';

export function SearchTestPageContent({ status, kbId }: { status: CapabilityStatus; kbId: string }) {
  const enabled = status === 'available';
  return <Stack spacing={3}>
    <Typography component="h2" variant="h5" fontWeight={750}>检索测试</Typography>
    {!enabled && <UnavailableState title="检索后端待接入" description="后端待接入；关键词、向量、RRF 和 Reranker 分阶段结果将在接口可用后展示。" capability="search" />}
    <Paper component="form" variant="outlined" sx={{ p: 3 }}>
      <Stack spacing={2}>
        <Typography color="text.secondary">知识库：{kbId}</Typography>
        <TextField label="检索问题" disabled={!enabled} />
        <TextField select label="检索模式" value="hybrid" disabled={!enabled}><MenuItem value="hybrid">混合检索</MenuItem></TextField>
        <Button variant="contained" disabled={!enabled}>执行检索测试</Button>
      </Stack>
    </Paper>
  </Stack>;
}
export function SearchTestPage() { const { kbId = '' } = useParams(); return <SearchTestPageContent status={capabilities.search} kbId={kbId} />; }
