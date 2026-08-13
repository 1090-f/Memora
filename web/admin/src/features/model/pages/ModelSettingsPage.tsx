import { useState } from 'react';
import { Button, Card, CardContent, Chip, Dialog, DialogActions, DialogContent, DialogTitle, IconButton, Stack, TextField, Typography } from '@mui/material';
import DeleteOutlineOutlined from '@mui/icons-material/DeleteOutlineOutlined';
import EditOutlined from '@mui/icons-material/EditOutlined';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { listModelConfigs, updateModelConfig, deleteModelConfig } from '../api';
import { CreateModelDialog } from '../components/CreateModelDialog';
import type { ModelConfig } from '../types';

export function ModelSettingsPageContent({ status }: { status: CapabilityStatus }) {
  const enabled = status === 'available';
  const queryClient = useQueryClient();
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [editModel, setEditModel] = useState<ModelConfig | null>(null);
  const [deleteModel, setDeleteModel] = useState<ModelConfig | null>(null);

  const query = useQuery({ queryKey: queryKeys.models, queryFn: () => listModelConfigs({ page: 1, page_size: 20 }), enabled });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<ModelConfig> }) => updateModelConfig(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.models });
      setEditModel(null);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteModelConfig(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.models });
      setDeleteModel(null);
    },
  });

  return <Stack spacing={3}>
    <Stack direction="row" alignItems="center">
      <Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>模型设置</Typography>
      <Button variant="contained" disabled={!enabled} onClick={() => setCreateDialogOpen(true)}>新增模型</Button>
    </Stack>
    {!enabled && <UnavailableState title="模型配置后端待接入" description="后端待接入；API Key 仅允许提交，服务端响应必须始终脱敏。" capability="model" />}
    {enabled && query.isPending && <LoadingState label="正在加载模型配置" />}
    {enabled && query.error && <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />}
    {query.data?.items.map((model) => <Card key={model.id} variant="outlined"><CardContent>
      <Stack direction="row" spacing={1} alignItems="center">
        <Typography variant="h6" sx={{ flexGrow: 1 }}>{model.name}</Typography>
        <Chip size="small" label={model.model_type} />
        {model.is_default && <Chip size="small" color="primary" label="默认" />}
        <IconButton size="small" onClick={() => setEditModel(model)}><EditOutlined /></IconButton>
        <IconButton size="small" color="error" onClick={() => setDeleteModel(model)}><DeleteOutlineOutlined /></IconButton>
      </Stack>
      <Typography color="text.secondary">{model.provider} · {model.base_url}</Typography>
      <Typography color="text.secondary" variant="body2">API Key: {model.api_key_masked}</Typography>
    </CardContent></Card>)}

    <CreateModelDialog open={createDialogOpen} onClose={() => setCreateDialogOpen(false)} />

    {/* 编辑对话框 */}
    <Dialog open={!!editModel} onClose={() => setEditModel(null)} fullWidth maxWidth="sm">
      <DialogTitle>编辑模型</DialogTitle>
      <DialogContent>
        {editModel && <EditModelForm model={editModel} onSave={(data) => updateMutation.mutate({ id: editModel.id, data })} />}
      </DialogContent>
      <DialogActions>
        <Button onClick={() => setEditModel(null)}>取消</Button>
      </DialogActions>
    </Dialog>

    {/* 删除确认对话框 */}
    <Dialog open={!!deleteModel} onClose={() => setDeleteModel(null)}>
      <DialogTitle>确认删除</DialogTitle>
      <DialogContent>
        <Typography>确定要删除模型 "{deleteModel?.name}" 吗？</Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => setDeleteModel(null)}>取消</Button>
        <Button color="error" onClick={() => deleteModel && deleteMutation.mutate(deleteModel.id)} disabled={deleteMutation.isPending}>
          {deleteMutation.isPending ? '删除中...' : '删除'}
        </Button>
      </DialogActions>
    </Dialog>
  </Stack>;
}

function EditModelForm({ model, onSave }: { model: ModelConfig; onSave: (data: Partial<ModelConfig>) => void }) {
  const [name, setName] = useState(model.name);
  const [provider, setProvider] = useState(model.provider);
  const [baseUrl, setBaseUrl] = useState(model.base_url);
  const [apiKey, setApiKey] = useState('');
  const [isDefault, setIsDefault] = useState(model.is_default);

  const handleSubmit = () => {
    const data: Partial<ModelConfig> = { name, provider, base_url: baseUrl, is_default: isDefault };
    if (apiKey) data.api_key = apiKey;
    onSave(data);
  };

  return (
    <Stack spacing={2} sx={{ mt: 1 }}>
      <TextField label="模型名称" value={name} onChange={(e) => setName(e.target.value)} fullWidth helperText="API 服务端支持的模型名，如 deepseek-chat" />
      <TextField label="Provider" value={provider} onChange={(e) => setProvider(e.target.value)} fullWidth />
      <TextField label="Base URL" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} fullWidth />
      <TextField label="API Key" value={apiKey} onChange={(e) => setApiKey(e.target.value)} fullWidth type="password" helperText="留空则不修改" />
      <Stack direction="row" alignItems="center">
        <input type="checkbox" id="is_default" checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} />
        <label htmlFor="is_default" style={{ marginLeft: 8 }}>设为默认</label>
      </Stack>
      <Button variant="contained" onClick={handleSubmit}>保存</Button>
    </Stack>
  );
}

export function ModelSettingsPage() { return <ModelSettingsPageContent status={capabilities.model} />; }
