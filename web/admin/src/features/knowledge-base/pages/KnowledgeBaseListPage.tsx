import AddOutlined from '@mui/icons-material/AddOutlined';
import DeleteOutlineOutlined from '@mui/icons-material/DeleteOutlineOutlined';
import EditOutlined from '@mui/icons-material/EditOutlined';
import MenuBookOutlined from '@mui/icons-material/MenuBookOutlined';
import QuestionAnswerOutlined from '@mui/icons-material/QuestionAnswerOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import SettingsOutlined from '@mui/icons-material/SettingsOutlined';
import { Alert, Box, Button, Card, CardActionArea, CardContent, Checkbox, Chip, Dialog, DialogActions, DialogContent, DialogTitle, FormControlLabel, IconButton, Stack, TextField, Typography } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { errorMessage } from '@/api/errors';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { createKnowledgeBase, deleteKnowledgeBase, updateKnowledgeBase, listKnowledgeBases } from '../api';
import type { KnowledgeBase } from '../types';

interface KnowledgeBaseForm {
  id: string | null;
  name: string;
  description: string;
  agent_enabled: boolean;
  network_enabled: boolean;
}

const emptyForm: KnowledgeBaseForm = { id: null, name: '', description: '', agent_enabled: true, network_enabled: false };

function KnowledgeBaseDialog({ open, form, onClose, onSaved }: {
  open: boolean;
  form: KnowledgeBaseForm;
  onClose: () => void;
  onSaved: (message: string) => void;
}) {
  const queryClient = useQueryClient();
  const [value, setValue] = useState<KnowledgeBaseForm>(form);

  useEffect(() => {
    if (open) setValue(form);
  }, [open, form]);

  const mutation = useMutation({
    mutationFn: (input: KnowledgeBaseForm) => {
      if (input.id) {
        return updateKnowledgeBase(input.id, {
          name: input.name,
          description: input.description || undefined,
          agent_enabled: input.agent_enabled,
          network_enabled: input.network_enabled,
        });
      }
      return createKnowledgeBase({
        name: input.name,
        description: input.description || undefined,
        agent_enabled: input.agent_enabled,
        network_enabled: input.network_enabled,
      });
    },
    onSuccess: (result, input) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBases });
      onSaved(input.id ? `知识库「${result.name}」已更新` : `知识库「${result.name}」已创建`);
      onClose();
    },
  });

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{form.id ? '编辑知识库' : '新建知识库'}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} mt={1}>
          <TextField
            label="名称"
            required
            value={value.name}
            onChange={(event) => setValue({ ...value, name: event.target.value })}
            error={value.name.trim() === ''}
            helperText={value.name.trim() === '' ? '请输入知识库名称' : undefined}
          />
          <TextField
            label="描述"
            multiline
            minRows={3}
            value={value.description}
            onChange={(event) => setValue({ ...value, description: event.target.value })}
          />
          <FormControlLabel
            control={<Checkbox checked={value.agent_enabled} onChange={(event) => setValue({ ...value, agent_enabled: event.target.checked })} />}
            label="启用 Agent 功能"
          />
          <FormControlLabel
            control={<Checkbox checked={value.network_enabled} onChange={(event) => setValue({ ...value, network_enabled: event.target.checked })} />}
            label="启用联网搜索"
          />
          {mutation.error && <Alert severity="error">{errorMessage(mutation.error)}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button
          variant="contained"
          disabled={value.name.trim() === '' || mutation.isPending}
          onClick={() => mutation.mutate({ ...value, name: value.name.trim() })}
        >
          {form.id ? '保存' : '创建'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function DeleteDialog({ open, kb, onClose, onDeleted }: {
  open: boolean;
  kb: KnowledgeBase | null;
  onClose: () => void;
  onDeleted: (message: string) => void;
}) {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: () => deleteKnowledgeBase(kb?.id as string),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBases });
      onDeleted(`知识库「${kb?.name}」已删除`);
      onClose();
    },
  });

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>删除知识库</DialogTitle>
      <DialogContent>
        <Typography>确定删除知识库「{kb?.name}」吗？其中的文档与索引将一并软删除，不可恢复。</Typography>
        {mutation.error && <Alert severity="error" sx={{ mt: 2 }}>{errorMessage(mutation.error)}</Alert>}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>取消</Button>
        <Button color="error" variant="contained" disabled={mutation.isPending} onClick={() => mutation.mutate()}>删除</Button>
      </DialogActions>
    </Dialog>
  );
}

export function KnowledgeBaseListContent({ status }: { status: CapabilityStatus }) {
  const enabled = status === 'available';
  const [dialogOpen, setDialogOpen] = useState(false);
  const [form, setForm] = useState<KnowledgeBaseForm>(emptyForm);
  const [deleteTarget, setDeleteTarget] = useState<KnowledgeBase | null>(null);
  const [notice, setNotice] = useState('');
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
        <Button
          variant="contained"
          startIcon={<AddOutlined />}
          disabled={!enabled}
          onClick={() => { setForm(emptyForm); setDialogOpen(true); }}
        >
          新建知识库
        </Button>
      </Stack>

      {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}

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
        <EmptyState
          title="还没有知识库"
          description="创建一个知识库后即可导入文档。"
          action={<Button variant="contained" startIcon={<AddOutlined />} onClick={() => { setForm(emptyForm); setDialogOpen(true); }}>新建知识库</Button>}
        />
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
              <Stack direction="row" alignItems="center" justifyContent="flex-end" px={1} pb={1}>
                <Button
                  component={Link}
                  to={`/chat/${kb.id}`}
                  size="small"
                  startIcon={<QuestionAnswerOutlined />}
                  aria-label={`开始问答 ${kb.name}`}
                  sx={{ mr: 'auto' }}
                >
                  开始问答
                </Button>
                <IconButton component={Link} to={`/kb/${kb.id}/search-test`} size="small" aria-label={`检索测试 ${kb.name}`}>
                  <SearchOutlined fontSize="small" />
                </IconButton>
                <IconButton component={Link} to={`/kb/${kb.id}/settings`} size="small" aria-label={`设置 ${kb.name}`}>
                  <SettingsOutlined fontSize="small" />
                </IconButton>
                <IconButton
                  size="small"
                  aria-label={`编辑 ${kb.name}`}
                  onClick={() => { setForm({ id: kb.id, name: kb.name, description: kb.description || '', agent_enabled: kb.agent_enabled, network_enabled: kb.network_enabled }); setDialogOpen(true); }}
                >
                  <EditOutlined fontSize="small" />
                </IconButton>
                <IconButton size="small" aria-label={`删除 ${kb.name}`} color="error" onClick={() => setDeleteTarget(kb)}>
                  <DeleteOutlineOutlined fontSize="small" />
                </IconButton>
              </Stack>
            </Card>
          ))}
        </Box>
      )}
      <KnowledgeBaseDialog open={dialogOpen} form={form} onClose={() => setDialogOpen(false)} onSaved={(message) => setNotice(message)} />
      <DeleteDialog open={deleteTarget !== null} kb={deleteTarget} onClose={() => setDeleteTarget(null)} onDeleted={(message) => setNotice(message)} />
    </Stack>
  );
}

export function KnowledgeBaseListPage() {
  return <KnowledgeBaseListContent status={capabilities.knowledgeBase} />;
}
