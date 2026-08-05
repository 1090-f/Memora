import { Button, Card, CardContent, Chip, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { listMemories } from '../api';

export function MemoryPageContent({ status }: { status: CapabilityStatus }) {
  const enabled = status === 'available';
  const query = useQuery({ queryKey: queryKeys.memories, queryFn: () => listMemories({ page: 1, page_size: 20 }), enabled });
  return (
    <Stack spacing={3}>
      <Stack direction="row" alignItems="center">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>长期记忆</Typography>
        <Button disabled={!enabled}>更改记忆状态</Button>
      </Stack>
      {!enabled && <UnavailableState title="长期记忆后端待接入" description="后端待接入；当前不会加载或修改由 Agent 形成的记忆。" capability="memory" />}
      {enabled && query.isPending && <LoadingState label="正在加载长期记忆" />}
      {enabled && query.error && <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />}
      {enabled && query.data?.items.length === 0 && <EmptyState title="暂无长期记忆" description="Agent 完成问答后可能自动提取记忆。" />}
      {query.data?.items.map((memory) => (
        <Card key={memory.id} variant="outlined"><CardContent>
          <Stack direction="row" spacing={1}><Chip size="small" label={memory.memory_type} /><Chip size="small" label={memory.status} /></Stack>
          <Typography mt={2} fontWeight={700}>{memory.summary}</Typography>
          <Typography color="text.secondary" sx={{ overflowWrap: 'anywhere' }}>{memory.content}</Typography>
        </CardContent></Card>
      ))}
    </Stack>
  );
}
export function MemoryPage() { return <MemoryPageContent status={capabilities.memory} />; }
