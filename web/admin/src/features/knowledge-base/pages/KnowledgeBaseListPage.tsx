import AddOutlined from '@mui/icons-material/AddOutlined';
import DeleteOutlineOutlined from '@mui/icons-material/DeleteOutlineOutlined';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import EditOutlined from '@mui/icons-material/EditOutlined';
import FolderOutlined from '@mui/icons-material/FolderOutlined';
import GridViewOutlined from '@mui/icons-material/GridViewOutlined';
import MenuBookOutlined from '@mui/icons-material/MenuBookOutlined';
import MoreVertOutlined from '@mui/icons-material/MoreVertOutlined';
import QuestionAnswerOutlined from '@mui/icons-material/QuestionAnswerOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import SettingsOutlined from '@mui/icons-material/SettingsOutlined';
import SmartToyOutlined from '@mui/icons-material/SmartToyOutlined';
import ViewListOutlined from '@mui/icons-material/ViewListOutlined';
import { Alert, Box, Button, CardActionArea, CardContent, Checkbox, Chip, Dialog, DialogActions, DialogContent, DialogTitle, FormControlLabel, IconButton, ListItemIcon, ListItemText, Menu, MenuItem, Select, Stack, TextField, ToggleButton, ToggleButtonGroup, Typography, type SelectChangeEvent } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState, type MouseEvent, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { errorMessage } from '@/api/errors';
import { EmptyState } from '@/components/shared/EmptyState';
import { ActionNotice } from '@/components/shared/ActionNotice';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { useAppSelector } from '@/store';
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

type ViewMode = 'grid' | 'list';
type SortMode = 'updated_desc' | 'updated_asc' | 'name_asc';

function formatDate(value: string): string {
  const match = value.match(/^\d{4}-\d{2}-\d{2}/);
  if (match) return match[0];
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date).replace(/\//g, '-');
}

function WelcomeIllustration() {
  return (
    <Box
      aria-hidden="true"
      sx={{
        position: 'absolute',
        inset: 0,
        left: '54%',
        overflow: 'hidden',
        opacity: { xs: 0.24, md: 0.8 },
        pointerEvents: 'none',
      }}
    >
      <Box sx={{ position: 'absolute', width: 410, height: 132, border: '2px solid rgba(111, 120, 255, .18)', borderRadius: '50%', right: 30, top: 42, transform: 'rotate(12deg)' }} />
      <Box sx={{ position: 'absolute', width: 470, height: 118, border: '18px solid rgba(112, 121, 255, .055)', borderRadius: '50%', right: -12, top: 54, transform: 'rotate(12deg)' }} />
      <Box sx={{ position: 'absolute', width: 220, height: 174, right: 90, top: 36 }}>
        <Box sx={{ position: 'absolute', inset: 0, clipPath: 'polygon(50% 25%, 100% 45%, 50% 67%, 0 45%)', background: 'linear-gradient(135deg, rgba(134,144,255,.74), rgba(91,83,255,.86))' }} />
        <Box sx={{ position: 'absolute', inset: 0, clipPath: 'polygon(0 45%, 50% 67%, 50% 100%, 0 78%)', background: 'linear-gradient(150deg, rgba(155,165,255,.44), rgba(116,128,242,.67))' }} />
        <Box sx={{ position: 'absolute', inset: 0, clipPath: 'polygon(50% 67%, 100% 45%, 100% 78%, 50% 100%)', background: 'linear-gradient(150deg, rgba(100,112,255,.79), rgba(69,78,218,.88))' }} />
        <Box sx={{ position: 'absolute', width: 58, height: 62, left: 84, top: 0 }}>
          <Box sx={{ position: 'absolute', inset: 0, clipPath: 'polygon(50% 0, 100% 20%, 50% 42%, 0 20%)', background: '#8c96ff' }} />
          <Box sx={{ position: 'absolute', inset: 0, clipPath: 'polygon(0 20%, 50% 42%, 50% 100%, 0 78%)', background: '#7583f0' }} />
          <Box sx={{ position: 'absolute', inset: 0, clipPath: 'polygon(50% 42%, 100% 20%, 100% 78%, 50% 100%)', background: '#6267eb' }} />
        </Box>
      </Box>
      <Typography sx={{ position: 'absolute', right: 92, top: 28, color: '#8190ff', fontSize: 31 }}>✦</Typography>
      <Typography sx={{ position: 'absolute', right: 315, bottom: 16, color: '#8190ff', fontSize: 22 }}>✦</Typography>
      <Typography sx={{ position: 'absolute', right: 410, top: 110, color: '#a2aaff', fontSize: 19 }}>✦</Typography>
    </Box>
  );
}

function StatItem({ icon, value, label }: {
  icon: ReactNode;
  value: number;
  label: string;
}) {
  return (
    <Stack direction="row" spacing={1.5} alignItems="center" sx={{ minWidth: { xs: 132, sm: 165 } }}>
      <Box sx={{ width: 46, height: 46, borderRadius: 2.5, display: 'grid', placeItems: 'center', color: '#4763f6', background: 'linear-gradient(135deg, rgba(77,105,255,.16), rgba(116,99,255,.06))' }}>
        {icon}
      </Box>
      <Box>
        <Typography sx={{ color: '#3458f5', fontSize: 22, fontWeight: 600, lineHeight: 1.05 }}>{value}</Typography>
        <Typography sx={{ color: '#5f6b85', fontSize: 14, mt: 0.55 }}>{label}</Typography>
      </Box>
    </Stack>
  );
}

export function KnowledgeBaseListContent({ status }: { status: CapabilityStatus }) {
  const enabled = status === 'available';
  const user = useAppSelector((state) => state.auth.user);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [form, setForm] = useState<KnowledgeBaseForm>(emptyForm);
  const [deleteTarget, setDeleteTarget] = useState<KnowledgeBase | null>(null);
  const [notice, setNotice] = useState('');
  const [viewMode, setViewMode] = useState<ViewMode>('grid');
  const [sortMode, setSortMode] = useState<SortMode>('updated_desc');
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [menuTarget, setMenuTarget] = useState<KnowledgeBase | null>(null);
  const query = useQuery({
    queryKey: queryKeys.knowledgeBases,
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 20, sort: 'updated_at_desc' }),
    enabled,
  });

  const items = useMemo(() => {
    const next = [...(query.data?.items ?? [])];
    next.sort((a, b) => {
      if (sortMode === 'name_asc') return a.name.localeCompare(b.name, 'zh-CN');
      const direction = sortMode === 'updated_desc' ? -1 : 1;
      return (new Date(a.updated_at).getTime() - new Date(b.updated_at).getTime()) * direction;
    });
    return next;
  }, [query.data?.items, sortMode]);

  const totalDocuments = items.reduce((total, kb) => total + kb.document_count, 0);
  const enabledAgents = items.filter((kb) => kb.agent_enabled).length;
  const displayName = user?.nickname || 'admin';

  const openMenu = (event: MouseEvent<HTMLElement>, kb: KnowledgeBase) => {
    event.preventDefault();
    event.stopPropagation();
    setMenuAnchor(event.currentTarget);
    setMenuTarget(kb);
  };

  const closeMenu = () => {
    setMenuAnchor(null);
    setMenuTarget(null);
  };

  const openEditor = (kb: KnowledgeBase) => {
    setForm({
      id: kb.id,
      name: kb.name,
      description: kb.description || '',
      agent_enabled: kb.agent_enabled,
      network_enabled: kb.network_enabled,
    });
    setDialogOpen(true);
  };

  const handleSortChange = (event: SelectChangeEvent<SortMode>) => {
    setSortMode(event.target.value as SortMode);
  };

  return (
    <Stack spacing={3.5} sx={{ width: '100%', maxWidth: 1440, mx: 'auto' }}>
      <Stack direction={{ xs: 'column', sm: 'row' }} gap={2} alignItems={{ xs: 'stretch', sm: 'center' }}>
        <Box sx={{ flexGrow: 1 }}>
          <Typography component="h2" sx={{ color: '#111c3a', fontSize: { xs: 26, md: 30 }, fontWeight: 700, lineHeight: 1.2 }}>我的知识库</Typography>
          <Typography sx={{ color: '#66728c', fontSize: 16, mt: 0.75 }}>管理和探索您的知识库，构建专属 Agent 能力。</Typography>
        </Box>
        <Button
          variant="contained"
          startIcon={<AddOutlined />}
          disabled={!enabled}
          onClick={() => { setForm(emptyForm); setDialogOpen(true); }}
          sx={{
            alignSelf: { xs: 'flex-start', sm: 'center' },
            minWidth: 162,
            height: 50,
            px: 2.5,
            borderRadius: 3,
            fontSize: 15,
            background: 'linear-gradient(135deg, #3867f4, #5d43f0)',
            boxShadow: '0 10px 24px rgba(70, 83, 235, .22)',
            '&:hover': { background: 'linear-gradient(135deg, #315de5, #5338df)', boxShadow: '0 12px 28px rgba(70, 83, 235, .3)' },
          }}
        >
          新建知识库
        </Button>
      </Stack>

      <ActionNotice message={notice} onClose={() => setNotice('')} />

      {enabled && query.data && (
        <Box
          sx={{
            position: 'relative',
            overflow: 'hidden',
            minHeight: 240,
            border: '1px solid rgba(101, 114, 245, .22)',
            borderRadius: 4,
            background: 'linear-gradient(110deg, rgba(228,235,255,.92), rgba(243,241,255,.84))',
            boxShadow: 'inset 0 1px 0 rgba(255,255,255,.8)',
            px: { xs: 3, md: 4 },
            py: 3.5,
          }}
        >
          <WelcomeIllustration />
          <Box sx={{ position: 'relative', zIndex: 1, maxWidth: { xs: '100%', md: '65%' } }}>
            <Typography sx={{ color: '#172343', fontSize: { xs: 23, md: 26 }, fontWeight: 700 }}>
              <Box component="span" sx={{ mr: 1 }}>👋</Box>
              欢迎回来，{displayName}
            </Typography>
            <Typography sx={{ color: '#5f6b85', fontSize: 16, mt: 1.25 }}>
              您已创建 {query.data.total} 个知识库，继续探索知识的边界吧！
            </Typography>
            <Stack direction="row" useFlexGap flexWrap="wrap" spacing={{ xs: 2, sm: 3.5 }} sx={{ mt: 4.5 }} divider={<Box sx={{ width: '1px', bgcolor: 'rgba(121, 134, 180, .18)' }} />}>
              <StatItem icon={<MenuBookOutlined />} value={query.data.total} label="知识库总数" />
              <StatItem icon={<DescriptionOutlined />} value={totalDocuments} label="文档总数" />
              <StatItem icon={<SmartToyOutlined />} value={enabledAgents} label="Agent 总数" />
            </Stack>
          </Box>
        </Box>
      )}

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
        <>
          <Stack direction="row" justifyContent="flex-end" alignItems="center" spacing={1.5} sx={{ mt: -0.5 }}>
            <ToggleButtonGroup
              exclusive
              value={viewMode}
              onChange={(_, value: ViewMode | null) => { if (value) setViewMode(value); }}
              aria-label="知识库视图"
              size="small"
              sx={{
                p: 0.4,
                bgcolor: '#f0f2f7',
                borderRadius: 2.5,
                '& .MuiToggleButton-root': { border: 0, borderRadius: '8px !important', color: '#647089', px: 1.25 },
                '& .Mui-selected': { bgcolor: '#fff !important', color: '#4163f5 !important', boxShadow: '0 2px 8px rgba(31,45,90,.1)' },
              }}
            >
              <ToggleButton value="grid" aria-label="网格视图"><GridViewOutlined fontSize="small" /></ToggleButton>
              <ToggleButton value="list" aria-label="列表视图"><ViewListOutlined fontSize="small" /></ToggleButton>
            </ToggleButtonGroup>
            <Select<SortMode>
              value={sortMode}
              onChange={handleSortChange}
              size="small"
              aria-label="知识库排序"
              sx={{
                minWidth: 160,
                height: 42,
                bgcolor: '#fff',
                borderRadius: 2.5,
                color: '#56627c',
                '& .MuiOutlinedInput-notchedOutline': { borderColor: '#dfe3eb' },
              }}
            >
              <MenuItem value="updated_desc">最近更新</MenuItem>
              <MenuItem value="updated_asc">最早更新</MenuItem>
              <MenuItem value="name_asc">名称排序</MenuItem>
            </Select>
          </Stack>

          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: viewMode === 'grid' ? { xs: '1fr', xl: 'repeat(2, minmax(0, 1fr))' } : '1fr',
              gap: 2.5,
            }}
          >
            {items.map((kb) => (
              <Box
                key={kb.id}
                sx={{
                  position: 'relative',
                  minHeight: viewMode === 'grid' ? 290 : 220,
                  overflow: 'hidden',
                  border: '1px solid #e2e5ec',
                  borderRadius: 4,
                  bgcolor: '#fff',
                  boxShadow: '0 8px 24px rgba(31, 45, 90, .035)',
                  transition: 'border-color .2s ease, box-shadow .2s ease, transform .2s ease',
                  '&:hover': { borderColor: 'rgba(75, 99, 245, .28)', boxShadow: '0 14px 34px rgba(31, 45, 90, .08)', transform: 'translateY(-2px)' },
                }}
              >
                <CardActionArea component={Link} to={`/kb/${kb.id}/docs`} sx={{ height: '100%', alignItems: 'stretch' }}>
                  <CardContent sx={{ height: '100%', minHeight: 'inherit', display: 'flex', flexDirection: 'column', p: 3, pr: 7, '&:last-child': { pb: 3 } }}>
                    <Box sx={{ width: 54, height: 54, borderRadius: 3, display: 'grid', placeItems: 'center', color: '#4762f5', background: 'linear-gradient(135deg, rgba(77,105,255,.16), rgba(116,99,255,.06))' }}>
                      <MenuBookOutlined />
                    </Box>
                    <Typography component="h3" sx={{ color: '#111c3a', fontSize: 21, fontWeight: 650, mt: 2.1 }}>{kb.name}</Typography>
                    <Typography sx={{ color: '#71809a', fontSize: 14, minHeight: 24, mt: 0.8 }}>{kb.description || '暂无描述'}</Typography>
                    <Stack direction="row" useFlexGap spacing={1} flexWrap="wrap" sx={{ mt: 2.25 }}>
                      <Chip
                        size="small"
                        icon={<DescriptionOutlined />}
                        label={`${kb.document_count} 个文档`}
                        sx={{ height: 29, bgcolor: '#f3f5f9', color: '#59667e', borderRadius: 2, '& .MuiChip-icon': { color: '#697baf', fontSize: 16 } }}
                      />
                      <Chip
                        size="small"
                        icon={<SmartToyOutlined />}
                        label={kb.agent_enabled ? 'Agent 已启用' : 'Agent 未启用'}
                        sx={{ height: 29, bgcolor: kb.agent_enabled ? '#e9f8f2' : '#f3f5f9', color: kb.agent_enabled ? '#24966d' : '#7a8498', borderRadius: 2, '& .MuiChip-icon': { color: 'inherit', fontSize: 16 } }}
                      />
                    </Stack>
                    <Stack direction="row" alignItems="center" sx={{ mt: 'auto', pt: 3 }}>
                      <Box sx={{ width: 27, height: 27, borderRadius: '50%', display: 'grid', placeItems: 'center', bgcolor: '#edf0ff', color: '#4762f5', fontSize: 12, mr: 1 }}>
                        {(displayName || 'A').trim().charAt(0).toUpperCase()}
                      </Box>
                      <Typography sx={{ color: '#6c7891', fontSize: 13 }}>{displayName}</Typography>
                      <Typography sx={{ ml: 'auto', color: '#8792a8', fontSize: 12 }}>更新于 {formatDate(kb.updated_at)}</Typography>
                    </Stack>
                  </CardContent>
                </CardActionArea>
                <IconButton
                  aria-label={`${kb.name} 更多操作`}
                  onClick={(event) => openMenu(event, kb)}
                  sx={{ position: 'absolute', right: 14, top: 14, color: '#18223d' }}
                >
                  <MoreVertOutlined />
                </IconButton>
              </Box>
            ))}
          </Box>
        </>
      )}

      <Menu
        anchorEl={menuAnchor}
        open={Boolean(menuAnchor && menuTarget)}
        onClose={closeMenu}
        slotProps={{ paper: { sx: { minWidth: 180, borderRadius: 2.5, boxShadow: '0 12px 36px rgba(31,45,90,.16)' } } }}
      >
        {menuTarget && <MenuItem component={Link} to={`/chat/${menuTarget.id}`} onClick={closeMenu}><ListItemIcon><QuestionAnswerOutlined fontSize="small" /></ListItemIcon><ListItemText>开始问答</ListItemText></MenuItem>}
        {menuTarget && <MenuItem component={Link} to={`/kb/${menuTarget.id}/docs`} onClick={closeMenu}><ListItemIcon><FolderOutlined fontSize="small" /></ListItemIcon><ListItemText>管理文档</ListItemText></MenuItem>}
        {menuTarget && <MenuItem component={Link} to={`/kb/${menuTarget.id}/search-test`} onClick={closeMenu}><ListItemIcon><SearchOutlined fontSize="small" /></ListItemIcon><ListItemText>检索测试</ListItemText></MenuItem>}
        {menuTarget && <MenuItem component={Link} to={`/kb/${menuTarget.id}/settings`} onClick={closeMenu}><ListItemIcon><SettingsOutlined fontSize="small" /></ListItemIcon><ListItemText>知识库设置</ListItemText></MenuItem>}
        {menuTarget && <MenuItem onClick={() => { const target = menuTarget; closeMenu(); openEditor(target); }}><ListItemIcon><EditOutlined fontSize="small" /></ListItemIcon><ListItemText>编辑</ListItemText></MenuItem>}
        {menuTarget && <MenuItem sx={{ color: 'error.main' }} onClick={() => { const target = menuTarget; closeMenu(); setDeleteTarget(target); }}><ListItemIcon><DeleteOutlineOutlined color="error" fontSize="small" /></ListItemIcon><ListItemText>删除</ListItemText></MenuItem>}
      </Menu>
      <KnowledgeBaseDialog open={dialogOpen} form={form} onClose={() => setDialogOpen(false)} onSaved={(message) => setNotice(message)} />
      <DeleteDialog open={deleteTarget !== null} kb={deleteTarget} onClose={() => setDeleteTarget(null)} onDeleted={(message) => setNotice(message)} />
    </Stack>
  );
}

export function KnowledgeBaseListPage() {
  return <KnowledgeBaseListContent status={capabilities.knowledgeBase} />;
}
