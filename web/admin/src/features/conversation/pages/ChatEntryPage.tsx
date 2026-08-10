import QuestionAnswerOutlined from '@mui/icons-material/QuestionAnswerOutlined';
import { Alert, Box, Button, Card, CardContent, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { queryKeys } from '@/api/queryKeys';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { listKnowledgeBases } from '@/features/knowledge-base/api';

export function ChatEntryPage() {
  const query = useQuery({
    queryKey: queryKeys.knowledgeBases,
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 100, sort: 'updated_at_desc' }),
  });

  return (
    <Stack spacing={3}>
      <Box>
        <Typography component="h2" variant="h5" fontWeight={750}>选择知识库</Typography>
        <Typography color="text.secondary">选择知识库后进入三栏智能问答工作台。</Typography>
      </Box>

      <Alert severity="info">
        智能问答页面入口已开放；当前后端尚未提供会话与 Agent SSE 路由，进入工作台后发送功能暂时禁用。
      </Alert>

      {query.isPending && <LoadingState label="正在加载知识库" />}
      {query.error && <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />}
      {query.data?.items.length === 0 && (
        <EmptyState
          title="还没有知识库"
          description="请先创建知识库并导入文档，再进入智能问答。"
          action={<Button component={Link} to="/knowledge-bases" variant="contained">前往知识库</Button>}
        />
      )}
      {query.data && query.data.items.length > 0 && (
        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: 2 }}>
          {query.data.items.map((kb) => (
            <Card key={kb.id} variant="outlined">
              <CardContent>
                <QuestionAnswerOutlined color="primary" />
                <Typography component="h3" variant="h6" mt={1}>{kb.name}</Typography>
                <Typography color="text.secondary" minHeight={48}>{kb.description || '暂无描述'}</Typography>
                <Button
                  component={Link}
                  to={`/chat/${kb.id}`}
                  variant="contained"
                  fullWidth
                  sx={{ mt: 2 }}
                >
                  进入问答
                </Button>
              </CardContent>
            </Card>
          ))}
        </Box>
      )}
    </Stack>
  );
}
