import { useState } from 'react';
import { Button, Card, CardContent, Chip, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { listModelConfigs } from '../api';
import { CreateModelDialog } from '../components/CreateModelDialog';

export function ModelSettingsPageContent({ status }: { status: CapabilityStatus }) {
  const enabled = status === 'available';
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const query = useQuery({ queryKey: queryKeys.models, queryFn: () => listModelConfigs({ page: 1, page_size: 20 }), enabled });
  return <Stack spacing={3}>
    <Stack direction="row" alignItems="center"><Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>模型设置</Typography><Button variant="contained" disabled={!enabled} onClick={() => setCreateDialogOpen(true)}>新增模型</Button></Stack>
    {!enabled && <UnavailableState title="模型配置后端待接入" description="后端待接入；API Key 仅允许提交，服务端响应必须始终脱敏。" capability="model" />}
    {enabled && query.isPending && <LoadingState label="正在加载模型配置" />}
    {enabled && query.error && <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />}
    {query.data?.items.map((model) => <Card key={model.id} variant="outlined"><CardContent><Stack direction="row" spacing={1} alignItems="center"><Typography variant="h6">{model.name}</Typography><Chip size="small" label={model.model_type} />{model.is_default && <Chip size="small" color="primary" label="默认" />}</Stack><Typography color="text.secondary">{model.provider} · {model.api_key_masked}</Typography></CardContent></Card>)}
    <CreateModelDialog open={createDialogOpen} onClose={() => setCreateDialogOpen(false)} />
  </Stack>;
}
export function ModelSettingsPage() { return <ModelSettingsPageContent status={capabilities.model} />; }
