import ArrowBackOutlined from '@mui/icons-material/ArrowBackOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import SettingsOutlined from '@mui/icons-material/SettingsOutlined';
import {
  Alert,
  Box,
  Button,
  Chip,
  Divider,
  MenuItem,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { useMutation } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Link, useParams } from 'react-router-dom';
import { errorMessage } from '@/api/errors';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { runSearchTest } from '../api';
import type { SearchMode, SearchResult, SearchTestResponse } from '../types';

const modeLabel: Record<SearchMode, string> = {
  keyword: '关键词检索',
  vector: '向量检索',
  hybrid: '混合检索（RRF + Reranker）',
};

function score(value?: number) {
  return value === undefined ? '—' : value.toFixed(4);
}

const matchLevelLabel: Record<NonNullable<SearchResult['match_level']>, string> = {
  exact: '精确短语',
  strong: '强词召回',
  weak: '弱词兜底',
};

function ResultTable({ items }: { items: SearchResult[] }) {
  if (items.length === 0) {
    return <Typography color="text.secondary" py={3} textAlign="center">没有匹配的知识片段</Typography>;
  }
  return (
    <TableContainer>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell width={56}>排名</TableCell>
            <TableCell width={190}>文档</TableCell>
            <TableCell>命中内容与引用</TableCell>
            <TableCell width={190}>评分</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {items.map((item, index) => (
            <TableRow key={`${item.chunk_id}-${index}`} sx={{ verticalAlign: 'top' }}>
              <TableCell>{item.final_rank ?? index + 1}</TableCell>
              <TableCell>
                <Typography fontWeight={700}>{item.document_title || item.citation.document_title || item.document_id}</Typography>
                <Typography variant="caption" color="text.secondary">索引 v{item.index_version} · Chunk {item.chunk_id.slice(0, 8)}</Typography>
              </TableCell>
              <TableCell>
                <Typography sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{item.content}</Typography>
                <Divider sx={{ my: 1 }} />
                <Typography variant="caption" color="text.secondary">
                  引用：{item.citation.quoted_text || '已定位到当前知识片段'}
                </Typography>
                {item.citation.source_location && (
                  <Typography variant="caption" color="text.secondary" display="block">
                    位置：{JSON.stringify(item.citation.source_location)}
                  </Typography>
                )}
              </TableCell>
              <TableCell>
                <Stack spacing={0.25}>
                  {item.match_level && (
                    <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                      <Chip
                        size="small"
                        color={item.low_confidence ? 'warning' : item.match_level === 'exact' ? 'success' : 'primary'}
                        label={matchLevelLabel[item.match_level]}
                      />
                      {item.low_confidence && <Chip size="small" color="warning" variant="outlined" label="低置信度" />}
                    </Stack>
                  )}
                  {item.matched_terms && item.matched_terms.length > 0 && (
                    <Typography variant="caption">命中词：{item.matched_terms.join(' / ')}</Typography>
                  )}
                  {item.coverage !== undefined && (
                    <Typography variant="caption">覆盖率：{Math.round(item.coverage * 100)}%</Typography>
                  )}
                  {item.recall_stage && <Typography variant="caption">召回阶段：{item.recall_stage}</Typography>}
                  <Typography variant="caption">关键词：{score(item.keyword_score)}</Typography>
                  <Typography variant="caption">向量：{score(item.vector_score)}</Typography>
                  <Typography variant="caption">RRF 排名：{item.rrf_rank ?? '—'}</Typography>
                  <Typography variant="caption">重排：{score(item.reranker_score)}</Typography>
                </Stack>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function ResultSummary({ result }: { result: SearchTestResponse }) {
  const stages = [
    ['关键词候选', result.keyword_results.length],
    ['向量候选', result.vector_results.length],
    ['RRF 融合', result.rrf_results.length],
    ['Reranker', result.reranked_results.length],
    ['最终结果', result.final_results.length],
  ] as const;
  return (
    <Stack spacing={2}>
      <Alert severity={result.knowledge_status === 'sufficient' ? 'success' : 'warning'}>
        {result.knowledge_status === 'sufficient'
          ? '知识充分：已找到可用于回答的依据。'
          : '知识不足：当前知识库没有足够的有效依据。'}
      </Alert>
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        {stages.map(([label, count]) => <Chip key={label} label={`${label} ${count}`} variant="outlined" />)}
        {Object.entries(result.timing).map(([name, value]) => (
          <Chip key={name} color="primary" variant="outlined" label={`${name}: ${value} ms`} />
        ))}
      </Stack>
      <Paper variant="outlined">
        <Box px={2} py={1.5}><Typography fontWeight={750}>最终检索结果</Typography></Box>
        <Divider />
        <ResultTable items={result.final_results} />
      </Paper>
    </Stack>
  );
}

export function SearchTestPageContent({ status, kbId }: { status: CapabilityStatus; kbId: string }) {
  const enabled = status === 'available';
  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<SearchMode>('hybrid');
  const [topK, setTopK] = useState(8);
  const mutation = useMutation({ mutationFn: () => runSearchTest(kbId, { query: query.trim(), mode, top_k: topK }) });

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (enabled && query.trim()) mutation.mutate();
  };

  return (
    <Stack spacing={3}>
      <Stack direction={{ xs: 'column', md: 'row' }} alignItems={{ md: 'center' }} gap={1}>
        <Box sx={{ flexGrow: 1 }}>
          <Typography component="h2" variant="h5" fontWeight={750}>RAG 检索测试</Typography>
          <Typography color="text.secondary">验证关键词、向量、RRF、Reranker 和 Citation 的完整检索链路。</Typography>
        </Box>
        <Button component={Link} to={`/kb/${kbId}/docs`} startIcon={<ArrowBackOutlined />}>返回文档</Button>
        <Button component={Link} to={`/kb/${kbId}/settings`} startIcon={<SettingsOutlined />}>检索配置</Button>
      </Stack>
      {!enabled && <UnavailableState title="检索后端待接入" description="检索接口当前不可用。" capability="search" />}
      <Paper component="form" variant="outlined" sx={{ p: 3 }} onSubmit={submit}>
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} alignItems={{ md: 'flex-start' }}>
          <TextField
            label="检索问题"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            disabled={!enabled || mutation.isPending}
            multiline
            minRows={2}
            fullWidth
            autoFocus
          />
          <TextField select label="检索模式" value={mode} onChange={(event) => setMode(event.target.value as SearchMode)} disabled={!enabled || mutation.isPending} sx={{ minWidth: 230 }}>
            {Object.entries(modeLabel).map(([value, label]) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
          </TextField>
          <TextField label="Top K" type="number" value={topK} onChange={(event) => setTopK(Math.max(1, Math.min(20, Number(event.target.value))))} inputProps={{ min: 1, max: 20 }} disabled={!enabled || mutation.isPending} sx={{ width: 110 }} />
          <Button type="submit" variant="contained" startIcon={<SearchOutlined />} disabled={!enabled || !query.trim() || mutation.isPending} sx={{ minWidth: 150, height: 56 }}>
            {mutation.isPending ? '检索中…' : '执行检索'}
          </Button>
        </Stack>
      </Paper>
      {mutation.error && <Alert severity="error">{errorMessage(mutation.error)}</Alert>}
      {mutation.data && <ResultSummary result={mutation.data} />}
    </Stack>
  );
}

export function SearchTestPage() {
  const { kbId = '' } = useParams();
  return <SearchTestPageContent status={capabilities.search} kbId={kbId} />;
}
