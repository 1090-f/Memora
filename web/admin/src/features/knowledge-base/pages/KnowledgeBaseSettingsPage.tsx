import { Alert, Button, Divider, MenuItem, Paper, Stack, Switch, TextField, Typography } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { errorMessage } from '@/api/errors';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { listModelConfigs } from '@/features/model/api';
import { getKnowledgeBase, updateKnowledgeBase } from '../api';

export function KnowledgeBaseSettingsContent({ status, kbId, embedded = false }: { status: CapabilityStatus; kbId: string; embedded?: boolean }) {
  const enabled = status === 'available';
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [agentEnabled, setAgentEnabled] = useState(true);
  const [networkEnabled, setNetworkEnabled] = useState(false);
  const [defaultChatModelId, setDefaultChatModelId] = useState('');
  const [defaultEmbeddingModelId, setDefaultEmbeddingModelId] = useState('');
  const [defaultRerankerModelId, setDefaultRerankerModelId] = useState('');
  const [duplicatePolicy, setDuplicatePolicy] = useState<'skip' | 'create_new'>('skip');
  const [notice, setNotice] = useState('');

  const kbQuery = useQuery({
    queryKey: ['knowledge-bases', kbId],
    queryFn: () => getKnowledgeBase(kbId),
    enabled,
  });
  const modelsQuery = useQuery({
    queryKey: queryKeys.models,
    queryFn: () => listModelConfigs(),
    enabled,
  });

  useEffect(() => {
    if (!kbQuery.data) return;
    setName(kbQuery.data.name);
    setDescription(kbQuery.data.description || '');
    setAgentEnabled(kbQuery.data.agent_enabled);
    setNetworkEnabled(kbQuery.data.network_enabled);
    setDefaultChatModelId(kbQuery.data.default_chat_model_id || '');
    setDefaultEmbeddingModelId(kbQuery.data.default_embedding_model_id || '');
    setDefaultRerankerModelId(kbQuery.data.default_reranker_model_id || '');
    setDuplicatePolicy(kbQuery.data.duplicate_policy ?? 'skip');
  }, [kbQuery.data]);

  const saveMutation = useMutation({
    mutationFn: () => updateKnowledgeBase(kbId, {
      name: name.trim(),
      description: description || undefined,
      agent_enabled: agentEnabled,
      network_enabled: networkEnabled,
      // 始终提交模型字段：空串表示清除默认模型（后端写 NULL），undefined 才表示不修改。
      default_chat_model_id: defaultChatModelId,
      default_embedding_model_id: defaultEmbeddingModelId,
      default_reranker_model_id: defaultRerankerModelId,
      duplicate_policy: duplicatePolicy,
    }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['knowledge-bases', kbId] });
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBases });
      setNotice('知识库设置已保存');
    },
  });

  if (!enabled) {
    return (
      <Stack spacing={3} maxWidth={embedded ? 960 : 760} mx={embedded ? 'auto' : undefined}>
        {!embedded && <Typography component="h2" variant="h5" fontWeight={750}>知识库设置</Typography>}
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

  if (kbQuery.isPending || modelsQuery.isPending) return <LoadingState label="正在加载知识库设置" />;
  if (kbQuery.error) return <ErrorState error={kbQuery.error as Error} onRetry={() => void kbQuery.refetch()} />;
  if (modelsQuery.error) return <ErrorState error={modelsQuery.error as Error} onRetry={() => void modelsQuery.refetch()} />;
  if (!kbQuery.data || !modelsQuery.data) return null;

  const models = modelsQuery.data.items;
  const chatModels = models.filter((model) => model.model_type === 'chat');
  const embeddingModels = models.filter((model) => model.model_type === 'embedding');
  const rerankerModels = models.filter((model) => model.model_type === 'reranker');

  return (
    <Stack spacing={3} maxWidth={embedded ? 960 : 760} mx={embedded ? 'auto' : undefined}>
      {!embedded && (
        <Stack direction="row" alignItems="center">
          <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>知识库设置</Typography>
        </Stack>
      )}
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
          <TextField select label="重复策略" value={duplicatePolicy} onChange={(event) => setDuplicatePolicy(event.target.value as 'skip' | 'create_new')} helperText="导入文件时，遇到重复内容按此策略处理">
            <MenuItem value="skip">重复内容跳过（推荐）</MenuItem>
            <MenuItem value="create_new">重复内容创建新文档</MenuItem>
          </TextField>
          <Divider />
          <Typography component="h3" variant="h6">RAG 默认模型</Typography>
          {embeddingModels.length === 0 && <Alert severity="warning">尚未配置 Embedding 模型，向量与混合检索将不可用。</Alert>}
          <TextField select label="默认 Chat 模型" value={defaultChatModelId} onChange={(event) => setDefaultChatModelId(event.target.value)} helperText="供后续问答与 Agent 使用">
            <MenuItem value="">未指定</MenuItem>
            {chatModels.map((model) => <MenuItem key={model.id} value={model.id}>{model.name} · {model.provider}</MenuItem>)}
          </TextField>
          <TextField select label="默认 Embedding 模型" value={defaultEmbeddingModelId} onChange={(event) => setDefaultEmbeddingModelId(event.target.value)} helperText="文档索引与向量检索必须使用兼容维度的模型">
            <MenuItem value="">使用用户默认模型</MenuItem>
            {embeddingModels.map((model) => <MenuItem key={model.id} value={model.id}>{model.name} · {model.provider}{model.vector_dimension ? ` · ${model.vector_dimension} 维` : ''}</MenuItem>)}
          </TextField>
          <TextField select label="默认 Reranker 模型" value={defaultRerankerModelId} onChange={(event) => setDefaultRerankerModelId(event.target.value)} helperText="未指定或调用失败时自动降级为 RRF 排序">
            <MenuItem value="">不指定（使用 RRF）</MenuItem>
            {rerankerModels.map((model) => <MenuItem key={model.id} value={model.id}>{model.name} · {model.provider}</MenuItem>)}
          </TextField>
          {saveMutation.error && <Alert severity="error">{errorMessage(saveMutation.error)}</Alert>}
          <Stack direction="row" justifyContent="flex-end">
            <Button variant="contained" disabled={name.trim() === '' || saveMutation.isPending} onClick={() => saveMutation.mutate()}>保存设置</Button>
          </Stack>
        </Stack>
      </Paper>
    </Stack>
  );
}

export function KnowledgeBaseSettingsPage() {
  const { kbId = '' } = useParams();
  return <KnowledgeBaseSettingsContent status={capabilities.knowledgeBase} kbId={kbId} />;
}
