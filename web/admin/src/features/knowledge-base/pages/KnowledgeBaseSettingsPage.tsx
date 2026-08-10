import { Alert, Button, Divider, Paper, Stack, Switch, TextField, Typography } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { errorMessage } from '@/api/errors';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { getKnowledgeBase, getSearchConfig, updateKnowledgeBase, updateSearchConfig } from '../api';
import type { SearchConfig } from '../types';

function intField(value: string | undefined): number | undefined {
  if (value === undefined || value === '') return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? Math.trunc(parsed) : undefined;
}

function SearchConfigForm({ kbId, config }: { kbId: string; config: SearchConfig }) {
  const queryClient = useQueryClient();
  const [values, setValues] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState('');
  const mutation = useMutation({
    mutationFn: () => updateSearchConfig(kbId, {
      keyword_top_k: intField(values.keyword_top_k),
      vector_top_k: intField(values.vector_top_k),
      rrf_k: intField(values.rrf_k),
      rrf_top_k: intField(values.rrf_top_k),
      reranker_top_k: intField(values.reranker_top_k),
      reranker_threshold: values.reranker_threshold === '' ? undefined : Number(values.reranker_threshold),
      minimum_effective_results: intField(values.minimum_effective_results),
    }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['knowledge-bases', kbId, 'search-config'] });
      setNotice('搜索配置已保存');
    },
  });

  useEffect(() => {
    setValues({
      keyword_top_k: String(config.keyword_top_k),
      vector_top_k: String(config.vector_top_k),
      rrf_k: String(config.rrf_k),
      rrf_top_k: String(config.rrf_top_k),
      reranker_top_k: String(config.reranker_top_k),
      reranker_threshold: config.reranker_threshold === null ? '' : String(config.reranker_threshold),
      minimum_effective_results: String(config.minimum_effective_results),
    });
  }, [config]);

  const setField = (key: string, value: string) => setValues((prev) => ({ ...prev, [key]: value }));

  return (
    <Paper variant="outlined" sx={{ p: 3 }}>
      <Stack spacing={2}>
        <Typography component="h3" variant="h6">检索配置</Typography>
        <Typography color="text.secondary" variant="body2">关键词与向量候选数、RRF 融合与重排序参数。</Typography>
        <TextField label="关键词候选数（keyword_top_k）" type="number" value={values.keyword_top_k ?? ''} onChange={(event) => setField('keyword_top_k', event.target.value)} helperText="1～200" />
        <TextField label="向量候选数（vector_top_k）" type="number" value={values.vector_top_k ?? ''} onChange={(event) => setField('vector_top_k', event.target.value)} helperText="1～200" />
        <TextField label="RRF 常数（rrf_k）" type="number" value={values.rrf_k ?? ''} onChange={(event) => setField('rrf_k', event.target.value)} helperText="大于 0，通常为 60" />
        <TextField label="RRF 融合保留条数（rrf_top_k）" type="number" value={values.rrf_top_k ?? ''} onChange={(event) => setField('rrf_top_k', event.target.value)} helperText="1～100" />
        <TextField label="重排序返回条数（reranker_top_k）" type="number" value={values.reranker_top_k ?? ''} onChange={(event) => setField('reranker_top_k', event.target.value)} helperText="1～20" />
        <TextField label="重排序阈值（reranker_threshold）" type="number" inputProps={{ step: 0.05 }} value={values.reranker_threshold ?? ''} onChange={(event) => setField('reranker_threshold', event.target.value)} helperText="0～1，留空表示不设阈值" />
        <TextField label="最低有效结果数（minimum_effective_results）" type="number" value={values.minimum_effective_results ?? ''} onChange={(event) => setField('minimum_effective_results', event.target.value)} helperText="检索结果少于该值时判定知识不足" />
        {mutation.error && <Alert severity="error">{errorMessage(mutation.error)}</Alert>}
        {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
        <Stack direction="row" justifyContent="flex-end">
          <Button variant="contained" disabled={mutation.isPending} onClick={() => mutation.mutate()}>保存检索配置</Button>
        </Stack>
      </Stack>
    </Paper>
  );
}

export function KnowledgeBaseSettingsContent({ status, kbId }: { status: CapabilityStatus; kbId: string }) {
  const enabled = status === 'available';
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [agentEnabled, setAgentEnabled] = useState(true);
  const [networkEnabled, setNetworkEnabled] = useState(false);
  const [notice, setNotice] = useState('');

  const kbQuery = useQuery({
    queryKey: ['knowledge-bases', kbId],
    queryFn: () => getKnowledgeBase(kbId),
    enabled,
  });
  const configQuery = useQuery({
    queryKey: ['knowledge-bases', kbId, 'search-config'],
    queryFn: () => getSearchConfig(kbId),
    enabled,
  });

  useEffect(() => {
    if (!kbQuery.data) return;
    setName(kbQuery.data.name);
    setDescription(kbQuery.data.description || '');
    setAgentEnabled(kbQuery.data.agent_enabled);
    setNetworkEnabled(kbQuery.data.network_enabled);
  }, [kbQuery.data]);

  const saveMutation = useMutation({
    mutationFn: () => updateKnowledgeBase(kbId, {
      name: name.trim(),
      description: description || undefined,
      agent_enabled: agentEnabled,
      network_enabled: networkEnabled,
    }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['knowledge-bases', kbId] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBases });
      setNotice('知识库设置已保存');
    },
  });

  if (!enabled) {
    return (
      <Stack spacing={3} maxWidth={760}>
        <Typography component="h2" variant="h5" fontWeight={750}>知识库设置</Typography>
        <UnavailableState
          title="配置接口待接入"
          description="搜索和 Agent 配置的字段已按 Memora 契约预留，后端可用前不会提交更改。"
          capability="knowledgeBase"
        />
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Stack spacing={2}>
            <TextField label="知识库名称" disabled />
            <TextField label="描述" multiline minRows={3} disabled />
            <Button variant="contained" disabled>保存设置</Button>
          </Stack>
        </Paper>
      </Stack>
    );
  }

  if (kbQuery.isPending || configQuery.isPending) return <LoadingState label="正在加载知识库设置" />;
  if (kbQuery.error) return <ErrorState error={kbQuery.error as Error} onRetry={() => void kbQuery.refetch()} />;
  if (configQuery.error) return <ErrorState error={configQuery.error as Error} onRetry={() => void configQuery.refetch()} />;
  if (!kbQuery.data || !configQuery.data) return null;

  return (
    <Stack spacing={3} maxWidth={760}>
      <Stack direction="row" alignItems="center">
        <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>知识库设置</Typography>
      </Stack>
      {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
      <Paper variant="outlined" sx={{ p: 3 }}>
        <Stack spacing={2}>
          <Typography component="h3" variant="h6">基础信息</Typography>
          <TextField label="知识库名称" value={name} onChange={(event) => setName(event.target.value)} error={name.trim() === ''} helperText={name.trim() === '' ? '名称不能为空' : undefined} />
          <TextField label="描述" multiline minRows={3} value={description} onChange={(event) => setDescription(event.target.value)} />
          <Stack direction="row" spacing={2}>
            <Stack direction="row" alignItems="center" spacing={1}>
              <Switch checked={agentEnabled} onChange={(event) => setAgentEnabled(event.target.checked)} inputProps={{ 'aria-label': '启用 Agent 功能' }} />
              <Typography>启用 Agent 功能</Typography>
            </Stack>
            <Stack direction="row" alignItems="center" spacing={1}>
              <Switch checked={networkEnabled} onChange={(event) => setNetworkEnabled(event.target.checked)} inputProps={{ 'aria-label': '启用联网搜索' }} />
              <Typography>启用联网搜索</Typography>
            </Stack>
          </Stack>
          {saveMutation.error && <Alert severity="error">{errorMessage(saveMutation.error)}</Alert>}
          <Stack direction="row" justifyContent="flex-end">
            <Button variant="contained" disabled={name.trim() === '' || saveMutation.isPending} onClick={() => saveMutation.mutate()}>保存设置</Button>
          </Stack>
        </Stack>
      </Paper>
      <Divider />
      <SearchConfigForm kbId={kbId} config={configQuery.data} />
    </Stack>
  );
}

export function KnowledgeBaseSettingsPage() {
  const { kbId = '' } = useParams();
  return <KnowledgeBaseSettingsContent status={capabilities.knowledgeBase} kbId={kbId} />;
}
