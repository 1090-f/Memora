import ArrowForwardOutlined from '@mui/icons-material/ArrowForwardOutlined';
import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import CodeOutlined from '@mui/icons-material/CodeOutlined';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import FilterListOutlined from '@mui/icons-material/FilterListOutlined';
import QueryBuilderOutlined from '@mui/icons-material/QueryBuilderOutlined';
import SmartToyOutlined from '@mui/icons-material/SmartToyOutlined';
import { Box, Button, Card, CardContent, Chip, MenuItem, Select, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { queryKeys } from '@/api/queryKeys';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { listKnowledgeBases } from '@/features/knowledge-base/api';
import { useAppSelector } from '@/store';

type SortMode = 'updated_desc' | 'updated_asc' | 'name_asc';

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' }).replace(/\//g, '-');
}

export function ChatEntryPage() {
  const user = useAppSelector((state) => state.auth.user);
  const [filter, setFilter] = useState<'all' | 'enabled'>('all');
  const [sort, setSort] = useState<SortMode>('updated_desc');
  const query = useQuery({
    queryKey: queryKeys.knowledgeBases,
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 100, sort: 'updated_at_desc' }),
  });

  const items = useMemo(() => {
    const next = (query.data?.items ?? []).filter((kb) => filter === 'all' || kb.agent_enabled);
    return [...next].sort((a, b) => {
      if (sort === 'name_asc') return a.name.localeCompare(b.name, 'zh-CN');
      const result = new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime();
      return sort === 'updated_desc' ? -result : result;
    });
  }, [filter, query.data?.items, sort]);

  return (
    <Stack spacing={3} sx={{ width: '100%', maxWidth: 1440, mx: 'auto' }}>
      <Stack direction={{ xs: 'column', sm: 'row' }} gap={2} alignItems={{ sm: 'center' }}>
        <Box sx={{ flexGrow: 1 }}>
          <Typography component="h2" sx={{ color: '#111c3a', fontSize: { xs: 26, md: 30 }, fontWeight: 700, lineHeight: 1.2 }}>选择知识库</Typography>
          <Typography sx={{ color: '#66728c', fontSize: 15, mt: 0.65 }}>选择一个知识库后，进入三栏智能问答工作台</Typography>
        </Box>
        <Select value={filter} onChange={(event) => setFilter(event.target.value as 'all' | 'enabled')} size="small" sx={{ minWidth: 150, height: 44, bgcolor: '#fff', borderRadius: 2.5 }}>
          <MenuItem value="all">全部知识库</MenuItem>
          <MenuItem value="enabled">Agent 已启用</MenuItem>
        </Select>
        <Select value={sort} onChange={(event) => setSort(event.target.value as SortMode)} size="small" IconComponent={FilterListOutlined} sx={{ minWidth: 150, height: 44, bgcolor: '#fff', borderRadius: 2.5 }}>
          <MenuItem value="updated_desc">按最近更新</MenuItem>
          <MenuItem value="updated_asc">按最早更新</MenuItem>
          <MenuItem value="name_asc">按名称排序</MenuItem>
        </Select>
      </Stack>

      <Box sx={{ position: 'relative', overflow: 'hidden', minHeight: 112, border: '1px solid rgba(98,110,245,.2)', borderRadius: 3.5, background: 'linear-gradient(105deg, rgba(234,240,255,.95), rgba(244,241,255,.92))', px: 3.5, py: 2.5 }}>
        <Stack direction="row" spacing={2.2} alignItems="center" sx={{ position: 'relative', zIndex: 1 }}>
          <Box sx={{ width: 62, height: 62, borderRadius: 3, display: 'grid', placeItems: 'center', color: '#fff', background: 'linear-gradient(145deg, #3868f5, #6544ee)', boxShadow: '0 12px 28px rgba(75,77,235,.25)' }}><AutoAwesomeOutlined /></Box>
          <Box>
            <Typography sx={{ color: '#4057ea', fontSize: 15, fontWeight: 700 }}>智能问答会直接创建 Agent 运行任务，并实时展示执行过程。</Typography>
            <Typography sx={{ color: '#64718a', fontSize: 13, mt: 0.65 }}>选择合适的知识库，让 AI 为你提供更准确、更高效的回答。</Typography>
          </Box>
        </Stack>
        <Box sx={{ position: 'absolute', width: 240, height: 100, right: 30, top: 8, opacity: 0.35, transform: 'rotate(-5deg)' }}>
          <Box sx={{ position: 'absolute', width: 130, height: 62, right: 40, top: 14, borderRadius: 3, bgcolor: '#fff', boxShadow: '0 12px 35px rgba(92,85,225,.2)', transform: 'rotate(8deg)' }} />
          <Box sx={{ position: 'absolute', width: 90, height: 44, right: 0, top: 3, borderRadius: 2, bgcolor: '#dfe3ff', transform: 'rotate(-8deg)' }} />
          <Typography sx={{ position: 'absolute', right: 185, top: 2, color: '#8993ff', fontSize: 22 }}>✦</Typography>
        </Box>
      </Box>

      {query.isPending && <LoadingState label="正在加载知识库" />}
      {query.error && <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />}
      {query.data?.items.length === 0 && (
        <EmptyState
          title="还没有知识库"
          description="请先创建知识库并导入文档，再进入智能问答。"
          action={<Button component={Link} to="/knowledge-bases" variant="contained">前往知识库</Button>}
        />
      )}
      {query.data && query.data.items.length > 0 && items.length === 0 && <EmptyState title="没有符合条件的知识库" description="请调整筛选条件后重试。" />}
      {items.length > 0 && (
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(2, minmax(0,1fr))', xl: 'repeat(3, minmax(0,1fr))' }, gap: 2.5 }}>
          {items.map((kb, index) => (
            <Card key={kb.id} variant="outlined" sx={{ borderRadius: 3.5, borderColor: '#e1e5ed', boxShadow: '0 10px 30px rgba(31,45,90,.045)', transition: 'transform .2s ease, box-shadow .2s ease', '&:hover': { transform: 'translateY(-3px)', boxShadow: '0 16px 38px rgba(31,45,90,.1)' } }}>
              <CardContent sx={{ p: 3, '&:last-child': { pb: 0 } }}>
                <Stack direction="row" justifyContent="space-between" alignItems="flex-start">
                  <Box sx={{ width: 80, height: 80, borderRadius: 3, display: 'grid', placeItems: 'center', color: '#fff', fontSize: 25, fontWeight: 700, background: index % 3 === 1 ? 'linear-gradient(145deg,#8172f5,#6552e8)' : index % 3 === 2 ? 'linear-gradient(145deg,#ffac63,#ff771e)' : 'linear-gradient(145deg,#65bbff,#3483ef)', boxShadow: '0 12px 24px rgba(72,99,232,.2)' }}>
                    {kb.icon || (index % 3 === 1 ? <SmartToyOutlined sx={{ fontSize: 40 }} /> : index % 3 === 2 ? <CodeOutlined sx={{ fontSize: 40 }} /> : kb.name.slice(0, 3))}
                  </Box>
                  <Chip size="small" label={kb.agent_enabled ? '●  Agent 已启用' : 'Agent 未启用'} sx={{ height: 28, bgcolor: kb.agent_enabled ? '#e8f7ec' : '#f1f3f7', color: kb.agent_enabled ? '#25924d' : '#7d8799', fontWeight: 600 }} />
                </Stack>
                <Typography component="h3" sx={{ color: '#111c3a', fontSize: 22, fontWeight: 700, mt: 2.4 }}>{kb.name}</Typography>
                <Typography sx={{ color: '#66728a', fontSize: 14, minHeight: 44, mt: 0.65 }}>{kb.description || '暂无描述'}</Typography>
                <Box sx={{ borderTop: '1px dashed #dfe3eb', mt: 1.8, pt: 1.8, display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 1 }}>
                  <Stack direction="row" spacing={0.8} alignItems="center"><DescriptionOutlined sx={{ color: '#7466f2', fontSize: 20 }} /><Box><Typography sx={{ color: '#7a869c', fontSize: 11 }}>文档数</Typography><Typography sx={{ color: '#46526a', fontSize: 13 }}>{kb.document_count}</Typography></Box></Stack>
                  <Stack direction="row" spacing={0.8} alignItems="center"><SmartToyOutlined sx={{ color: '#2dab63', fontSize: 20 }} /><Box><Typography sx={{ color: '#7a869c', fontSize: 11 }}>Agent</Typography><Typography sx={{ color: '#46526a', fontSize: 13 }}>{kb.agent_enabled ? '已启用' : '未启用'}</Typography></Box></Stack>
                  <Stack direction="row" spacing={0.8} alignItems="center"><QueryBuilderOutlined sx={{ color: '#8491a8', fontSize: 20 }} /><Box><Typography sx={{ color: '#7a869c', fontSize: 11 }}>最近更新</Typography><Typography sx={{ color: '#46526a', fontSize: 13 }}>{formatDate(kb.updated_at)}</Typography></Box></Stack>
                </Box>
                <Button
                  component={Link}
                  to={`/chat/${kb.id}`}
                  variant="contained"
                  fullWidth
                  endIcon={<ArrowForwardOutlined />}
                  sx={{ mt: 2.2, height: 47, borderRadius: 2.2, background: 'linear-gradient(135deg,#4e58df,#5b46e8)', boxShadow: '0 8px 18px rgba(74,70,220,.22)' }}
                >
                  进入问答
                </Button>
                <Stack direction="row" alignItems="center" sx={{ mx: -3, mt: 2.2, px: 3, py: 1.3, borderTop: '1px solid #e7eaf0' }}>
                  <Box sx={{ width: 25, height: 25, borderRadius: '50%', display: 'grid', placeItems: 'center', bgcolor: '#4d74f5', color: '#fff', fontSize: 11 }}>{(user?.nickname || 'A').charAt(0).toUpperCase()}</Box>
                  <Typography sx={{ color: '#748097', fontSize: 12, ml: 1 }}>{user?.nickname || 'admin'}</Typography>
                  <Typography sx={{ color: '#8792a6', fontSize: 11.5, ml: 'auto' }}>{formatDate(kb.updated_at)} 更新</Typography>
                </Stack>
              </CardContent>
            </Card>
          ))}
        </Box>
      )}
    </Stack>
  );
}
