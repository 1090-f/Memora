import ExpandMoreOutlined from '@mui/icons-material/ExpandMoreOutlined';
import TuneOutlined from '@mui/icons-material/TuneOutlined';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { getSearchConfig, updateSearchConfig } from '@/features/knowledge-base/api';
import { listModelConfigs } from '@/features/model/api';

function intField(value: string | undefined): number | undefined {
  if (value === undefined || value === '') return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? Math.trunc(parsed) : undefined;
}

export function SearchConfigPanel({ kbId }: { kbId: string }) {
  const queryClient = useQueryClient();
  const [values, setValues] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState('');
  const configQuery = useQuery({
    queryKey: queryKeys.knowledgeBaseSearchConfig(kbId),
    queryFn: () => getSearchConfig(kbId),
  });
  const modelsQuery = useQuery({
    queryKey: queryKeys.models,
    queryFn: () => listModelConfigs(),
  });
  const mutation = useMutation({
    mutationFn: () => updateSearchConfig(kbId, {
      keyword_top_k: intField(values.keyword_top_k),
      vector_top_k: intField(values.vector_top_k),
      min_vector_score: values.min_vector_score === '' ? undefined : Number(values.min_vector_score),
      rrf_k: intField(values.rrf_k),
      rrf_top_k: intField(values.rrf_top_k),
      reranker_top_k: intField(values.reranker_top_k),
      reranker_threshold: values.reranker_threshold === '' ? undefined : Number(values.reranker_threshold),
      minimum_effective_results: intField(values.minimum_effective_results),
      reranker_model_id: values.reranker_model_id || undefined,
    }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBaseSearchConfig(kbId) });
      setNotice('检索参数已保存，可以立即重新测试');
    },
  });

  useEffect(() => {
    const config = configQuery.data;
    if (!config) return;
    setValues({
      keyword_top_k: String(config.keyword_top_k),
      vector_top_k: String(config.vector_top_k),
      min_vector_score: config.min_vector_score === null ? '' : String(config.min_vector_score),
      rrf_k: String(config.rrf_k),
      rrf_top_k: String(config.rrf_top_k),
      reranker_top_k: String(config.reranker_top_k),
      reranker_threshold: config.reranker_threshold === null ? '' : String(config.reranker_threshold),
      minimum_effective_results: String(config.minimum_effective_results),
      reranker_model_id: config.reranker_model_id || '',
    });
  }, [configQuery.data]);

  const setField = (key: string, value: string) => setValues((previous) => ({ ...previous, [key]: value }));
  const rerankerModels = modelsQuery.data?.items.filter((model) => model.model_type === 'reranker') ?? [];
  const loading = configQuery.isPending || modelsQuery.isPending;

  return (
    <Accordion variant="outlined" disableGutters sx={{ borderRadius: '12px !important', '&::before': { display: 'none' } }}>
      <AccordionSummary expandIcon={<ExpandMoreOutlined />} sx={{ minHeight: 64, px: 2.5 }}>
        <Stack direction="row" spacing={1.25} alignItems="center">
          <Box sx={{ width: 34, height: 34, display: 'grid', placeItems: 'center', borderRadius: 2, bgcolor: '#eef1ff', color: '#5263ef' }}>
            <TuneOutlined fontSize="small" />
          </Box>
          <Box>
            <Typography fontWeight={700}>检索参数</Typography>
            <Typography variant="body2" color="text.secondary">调整候选召回、融合和重排序规则，保存后重新执行检索查看效果。</Typography>
          </Box>
        </Stack>
      </AccordionSummary>
      <AccordionDetails sx={{ px: 2.5, pb: 2.5, pt: 0.5 }}>
        {loading && <Typography color="text.secondary">正在加载检索参数…</Typography>}
        {configQuery.error && <Alert severity="error">检索参数加载失败：{errorMessage(configQuery.error)}</Alert>}
        {modelsQuery.error && <Alert severity="warning">重排序模型加载失败：{errorMessage(modelsQuery.error)}</Alert>}
        {!loading && configQuery.data && (
          <Stack spacing={2}>
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(2, minmax(0, 1fr))' }, gap: 2 }}>
              <TextField label="关键词候选数" type="number" value={values.keyword_top_k ?? ''} onChange={(event) => setField('keyword_top_k', event.target.value)} helperText="关键词检索最多召回 1～200 条" />
              <TextField label="向量候选数" type="number" value={values.vector_top_k ?? ''} onChange={(event) => setField('vector_top_k', event.target.value)} helperText="向量检索最多召回 1～200 条" />
              <TextField label="最低向量相似度" type="number" inputProps={{ step: 0.05, min: 0, max: 1 }} value={values.min_vector_score ?? ''} onChange={(event) => setField('min_vector_score', event.target.value)} helperText="0～1，低于该分数的向量结果被过滤；0 表示不启用" />
              <TextField label="RRF 融合常数" type="number" value={values.rrf_k ?? ''} onChange={(event) => setField('rrf_k', event.target.value)} helperText="用于平衡不同召回通道，通常为 60" />
              <TextField label="融合结果数" type="number" value={values.rrf_top_k ?? ''} onChange={(event) => setField('rrf_top_k', event.target.value)} helperText="融合阶段保留 1～100 条" />
              <TextField label="重排序结果数" type="number" value={values.reranker_top_k ?? ''} onChange={(event) => setField('reranker_top_k', event.target.value)} helperText="重排序后最多保留 1～20 条" />
              <TextField label="最低重排序分数" type="number" inputProps={{ step: 0.05 }} value={values.reranker_threshold ?? ''} onChange={(event) => setField('reranker_threshold', event.target.value)} helperText="0～1，留空表示不限制" />
              <TextField label="知识充分最低结果数" type="number" value={values.minimum_effective_results ?? ''} onChange={(event) => setField('minimum_effective_results', event.target.value)} helperText="低于该数量时提示知识不足" />
              <TextField select label="重排序模型" value={values.reranker_model_id ?? ''} onChange={(event) => setField('reranker_model_id', event.target.value)} helperText="留空时使用知识库默认模型">
                <MenuItem value="">使用知识库默认模型</MenuItem>
                {rerankerModels.map((model) => <MenuItem key={model.id} value={model.id}>{model.name} · {model.provider}</MenuItem>)}
              </TextField>
            </Box>
            {mutation.error && <Alert severity="error">保存失败：{errorMessage(mutation.error)}</Alert>}
            {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
            <Stack direction="row" justifyContent="flex-end">
              <Button variant="contained" disabled={mutation.isPending} onClick={() => mutation.mutate()}>
                {mutation.isPending ? '正在保存…' : '保存检索参数'}
              </Button>
            </Stack>
          </Stack>
        )}
      </AccordionDetails>
    </Accordion>
  );
}
