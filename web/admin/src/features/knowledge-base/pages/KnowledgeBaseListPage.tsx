import AddOutlined from '@mui/icons-material/AddOutlined';
import MenuBookOutlined from '@mui/icons-material/MenuBookOutlined';
import { Box, Button, Card, CardActionArea, CardContent, Chip, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { listKnowledgeBases } from '../api';

export function KnowledgeBaseListContent({ status }: { status: CapabilityStatus }) {
  const enabled = status === 'available';
  const query = useQuery({
    queryKey: queryKeys.knowledgeBases,
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 20, sort: 'updated_at_desc' }),
    enabled,
  });

  return (
    <Stack spacing={3}>
      <Stack direction="row" alignItems="center">
        <Box sx={{ flexGrow: 1 }}>
          <Typography component="h2" variant="h5" fontWeight={750}>我的知识库</Typography>
          <Typography color="text.secondary">每个知识库拥有独立的文档、检索与 Agent 配置。</Typography>
        </Box>
        <Button variant="contained" startIcon={<AddOutlined />} disabled={!enabled}>新建知识库</Button>
      </Stack>

      {!enabled && (
        <UnavailableState
          title="知识库后端待接入"
          description="后端尚未提供知识库接口；当前不会发起请求，新建与编辑操作已禁用。"
          capability="knowledgeBase"
        />
      )}
      {enabled && query.isPending && <LoadingState label="正在加载知识库" />}
      {enabled && query.error && <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />}
      {enabled && query.data?.items.length === 0 && (
        <EmptyState title="还没有知识库" description="创建一个知识库后即可导入文档。" />
      )}
      {enabled && query.data && query.data.items.length > 0 && (
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: 2 }}>
          {query.data.items.map((kb) => (
            <Card key={kb.id} variant="outlined">
              <CardActionArea component={Link} to={`/kb/${kb.id}/docs`} sx={{ height: '100%' }}>
                <CardContent>
                  <MenuBookOutlined color="primary" />
                  <Typography component="h3" variant="h6" mt={1}>{kb.name}</Typography>
                  <Typography color="text.secondary" minHeight={48}>{kb.description || '暂无描述'}</Typography>
                  <Stack direction="row" spacing={1} mt={2} flexWrap="wrap">
                    <Chip size="small" label={`${kb.document_count} 篇文档`} />
                    <Chip size="small" label={kb.agent_enabled ? 'Agent 已启用' : 'Agent 未启用'} />
                  </Stack>
                </CardContent>
              </CardActionArea>
            </Card>
          ))}
        </Box>
      )}
    </Stack>
  );
}

export function KnowledgeBaseListPage() {
  return <KnowledgeBaseListContent status={capabilities.knowledgeBase} />;
}
