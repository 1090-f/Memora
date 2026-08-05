import { Button, Card, CardContent, Chip, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { listMcpServers } from '../api';

export function McpPageContent({ status }: { status: CapabilityStatus }) {
  const enabled = status === 'available';
  const query = useQuery({ queryKey: queryKeys.mcpServers, queryFn: () => listMcpServers({ page: 1, page_size: 20 }), enabled });
  return <Stack spacing={3}>
    <Stack direction="row" alignItems="center"><Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>MCP 工具</Typography><Button variant="contained" disabled={!enabled}>新增 MCP 服务</Button></Stack>
    {!enabled && <UnavailableState title="MCP 后端待接入" description="后端待接入；仅允许只读工具，密钥与敏感 Header 不会在页面明文回显。" capability="mcp" />}
    {enabled && query.isPending && <LoadingState label="正在加载 MCP 服务" />}
    {enabled && query.error && <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />}
    {query.data?.items.map((server) => <Card key={server.id} variant="outlined"><CardContent><Stack direction="row" justifyContent="space-between"><Typography variant="h6">{server.name}</Typography><Chip size="small" label={server.enabled ? '已启用' : '已停用'} /></Stack><Typography color="text.secondary" sx={{ overflowWrap: 'anywhere' }}>{server.url}</Typography></CardContent></Card>)}
  </Stack>;
}
export function McpPage() { return <McpPageContent status={capabilities.mcp} />; }
