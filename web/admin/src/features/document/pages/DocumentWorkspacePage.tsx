import CachedOutlined from '@mui/icons-material/CachedOutlined';
import CheckCircleOutlineRounded from '@mui/icons-material/CheckCircleOutlineRounded';
import CloseOutlined from '@mui/icons-material/CloseOutlined';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import ErrorOutlineRounded from '@mui/icons-material/ErrorOutlineRounded';
import MoreVertOutlined from '@mui/icons-material/MoreVertOutlined';
import PendingActionsOutlined from '@mui/icons-material/PendingActionsOutlined';
import PictureAsPdfOutlined from '@mui/icons-material/PictureAsPdfOutlined';
import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import SettingsOutlined from '@mui/icons-material/SettingsOutlined';
import UploadFileOutlined from '@mui/icons-material/UploadFileOutlined';
import VisibilityOutlined from '@mui/icons-material/VisibilityOutlined';
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  InputAdornment,
  MenuItem,
  Pagination,
  Paper,
  Select,
  Stack,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Link, useParams } from 'react-router-dom';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { getKnowledgeBase, getKnowledgeBaseDashboard } from '@/features/knowledge-base/api';
import type { KnowledgeBaseDashboard } from '@/features/knowledge-base/types';
import { KnowledgeBaseSettingsContent } from '@/features/knowledge-base/pages/KnowledgeBaseSettingsPage';
import { SearchTestPageContent } from '@/features/search/pages/SearchTestPage';
import {
  createDirectory,
  cleanupImportTasks,
  deleteDocument,
  getDirectoryTree,
  getDocument,
  getDocumentProcessing,
  listDocuments,
  listImportTasks,
  reindexDocument,
  retryDocumentProcessing,
  retryImportTask,
  startImportTask,
} from '../api';
import { DocumentViewer } from '../components/DocumentViewer';
import { ImportDrawer } from '../components/ImportDrawer';
import { KnowledgeTree } from '../components/KnowledgeTree';
import {
  documentStatusFilterParams,
  documentStatusOptions,
  type DocumentStatusFilter,
} from '../status';
import type {
  DocumentListItem,
  DocumentProcessingStatus,
  DocumentSourceType,
  ImportTask,
} from '../types';

const sourceLabel: Record<Exclude<DocumentSourceType, 'manual'>, string> = {
  file: '文件',
  url: 'URL',
};

const taskLabel: Record<ImportTask['status'], string> = {
  pending: '等待中',
  running: '进行中',
  succeeded: '已完成',
  failed: '失败',
  skipped: '已跳过',
};

const activeProcessingStatuses: DocumentProcessingStatus[] = [
  'pending',
  'parsing',
  'cleaning',
  'chunking',
  'embedding',
  'keyword_indexing',
];

const activeRefreshInterval = 2000;
const idleRefreshInterval = 5000;
const pageSize = 10;
type WorkspaceTab = 'documents' | 'imports';

function WorkspaceFeatureDialog({ open, onClose, icon, title, description, children }: {
  open: boolean;
  onClose: () => void;
  icon: ReactNode;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="xl"
      scroll="paper"
      slotProps={{
        paper: {
          sx: {
            width: { xs: 'calc(100% - 24px)', md: 'min(1180px, calc(100% - 96px))' },
            height: { xs: 'calc(100% - 24px)', md: 'min(82vh, 860px)' },
            maxHeight: { xs: 'calc(100% - 24px)', md: 'calc(100% - 64px)' },
            borderRadius: { xs: 2.5, md: 3.5 },
            border: '1px solid #e1e5ed',
            boxShadow: '0 24px 72px rgba(25, 38, 82, .24)',
          },
        },
        backdrop: { sx: { bgcolor: 'rgba(29, 42, 76, .32)', backdropFilter: 'blur(1px)' } },
      }}
    >
      <DialogTitle sx={{ px: { xs: 2, md: 3 }, py: 2, borderBottom: '1px solid', borderColor: 'divider' }}>
        <Stack direction="row" alignItems="center" spacing={1.5}>
          <Box sx={{ width: 38, height: 38, display: 'grid', placeItems: 'center', borderRadius: '50%', bgcolor: '#eef0ff', color: '#5361ee' }}>{icon}</Box>
          <Box sx={{ flexGrow: 1, minWidth: 0 }}>
            <Typography component="h2" variant="h6" fontWeight={750}>{title}</Typography>
            <Typography variant="body2" color="text.secondary">{description}</Typography>
          </Box>
          <IconButton aria-label={`关闭${title}`} onClick={onClose}><CloseOutlined /></IconButton>
        </Stack>
      </DialogTitle>
      <DialogContent sx={{ p: { xs: 2, md: 3 }, bgcolor: '#f8faff' }}>
        {children}
      </DialogContent>
    </Dialog>
  );
}

function isProcessing(status?: DocumentProcessingStatus) {
  return status !== undefined && activeProcessingStatuses.includes(status);
}

function MetricItem({ label, value, detail, tone = 'default' }: {
  label: string;
  value: ReactNode;
  detail: ReactNode;
  tone?: 'default' | 'success' | 'warning' | 'error';
}) {
  const color = tone === 'success' ? '#229a48' : tone === 'warning' ? '#dc7a19' : tone === 'error' ? '#d84c4c' : '#172343';
  return (
    <Box sx={{ minWidth: 0, px: { xs: 1.5, md: 2.25 }, py: 1.7, borderLeft: { xs: 0, sm: '1px solid #e8ebf1' }, '&:first-of-type': { borderLeft: 0 } }}>
      <Typography sx={{ color: '#68758e', fontSize: 12.5, mb: 0.35 }}>{label}</Typography>
      <Typography component="div" sx={{ color, fontSize: 24, fontWeight: 750, lineHeight: 1.2 }}>{value}</Typography>
      <Typography component="div" sx={{ color: '#8390a6', fontSize: 11.5, mt: 0.55 }}>{detail}</Typography>
    </Box>
  );
}

function ImportTrendChart({ data }: { data: KnowledgeBaseDashboard['import_trend'] }) {
  const values = data.map((point) => point.count);
  const max = Math.max(1, ...values);
  const points = data.map((point, index) => {
    const x = data.length <= 1 ? 320 : 20 + (index * 600) / (data.length - 1);
    const y = 96 - (point.count / max) * 76;
    return { ...point, x, y };
  });
  return (
    <Paper variant="outlined" sx={{ p: 2.1, borderRadius: 3, borderColor: '#e3e7ef', minHeight: 184 }}>
      <Typography sx={{ color: '#26324d', fontSize: 13, fontWeight: 700 }}>近 7 天导入趋势</Typography>
      <Box sx={{ mt: 1.2, width: '100%', overflow: 'hidden' }}>
        <svg viewBox="0 0 640 122" width="100%" height="122" role="img" aria-label="近 7 天导入任务趋势">
          {[20, 58, 96].map((y) => <line key={y} x1="20" x2="620" y1={y} y2={y} stroke="#edf0f5" strokeWidth="1" />)}
          {points.length > 1 && <polyline points={points.map((point) => `${point.x},${point.y}`).join(' ')} fill="none" stroke="#4f67f6" strokeWidth="2.5" strokeLinejoin="round" strokeLinecap="round" />}
          {points.map((point) => <circle key={point.date} cx={point.x} cy={point.y} r="4" fill="#4f67f6" stroke="#fff" strokeWidth="2" />)}
        </svg>
        <Box sx={{ display: 'grid', gridTemplateColumns: `repeat(${Math.max(1, data.length)}, 1fr)`, mt: -2.6 }}>
          {data.map((point) => <Typography key={point.date} align="center" sx={{ color: '#7f8ba1', fontSize: 10.5 }}>{point.date.slice(5)}</Typography>)}
        </Box>
      </Box>
    </Paper>
  );
}

function RecentActivities({ data }: { data: KnowledgeBaseDashboard['recent_activities'] }) {
  return (
    <Paper variant="outlined" sx={{ p: 2.1, borderRadius: 3, borderColor: '#e3e7ef', minHeight: 184 }}>
      <Typography sx={{ color: '#26324d', fontSize: 13, fontWeight: 700, mb: 1.1 }}>近期活动</Typography>
      {data.length === 0 ? (
        <Typography sx={{ color: '#8390a6', fontSize: 13, py: 4, textAlign: 'center' }}>暂无导入活动</Typography>
      ) : (
        <Stack spacing={0.9}>
          {data.map((activity) => {
            const failed = activity.status === 'failed';
            const running = activity.status === 'running' || activity.status === 'pending';
            return (
              <Stack key={activity.id} direction="row" spacing={1.2} alignItems="center" sx={{ minHeight: 28 }}>
                <Box sx={{ width: 7, height: 7, borderRadius: '50%', bgcolor: failed ? '#e35c5c' : running ? '#4f67f6' : '#35aa59', flexShrink: 0 }} />
                <Typography sx={{ color: '#33405b', fontSize: 12.5, fontWeight: 600, minWidth: 100 }}>{activity.title}</Typography>
                <Typography noWrap sx={{ color: '#71809a', fontSize: 12, flexGrow: 1 }}>{activity.description}</Typography>
                <Typography sx={{ color: '#8b96a9', fontSize: 11.5, flexShrink: 0 }}>{new Date(activity.occurred_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</Typography>
              </Stack>
            );
          })}
        </Stack>
      )}
    </Paper>
  );
}

function DocumentList({
  kbId,
  directoryId,
  keyword,
  status,
  sourceType,
  selectedId,
  onSelect,
  onDelete,
  onRetry,
  onReindex,
  onDocumentsChange,
}: {
  kbId: string;
  directoryId: string | null;
  keyword: string;
  status: DocumentStatusFilter;
  sourceType: '' | DocumentSourceType;
  selectedId: string | null;
  onSelect: (id: string) => void;
  onDelete: (document: DocumentListItem) => void;
  onRetry: (document: DocumentListItem) => void;
  onReindex: (document: DocumentListItem) => void;
  onDocumentsChange?: (items: DocumentListItem[], total: number) => void;
}) {
  const [page, setPage] = useState(1);
  const query = useQuery({
    queryKey: [...queryKeys.documents(kbId), { directoryId, keyword, status, sourceType, page }],
    queryFn: () => listDocuments(kbId, {
      page,
      page_size: pageSize,
      directory_id: directoryId || undefined,
      keyword: keyword.trim() || undefined,
      ...documentStatusFilterParams(status),
      source_type: sourceType || undefined,
    }),
    // reindex 接口返回时仍可能读到旧终态；空闲期也保持低频刷新以跨过 worker 接单竞态。
    refetchInterval: (result) => result.state.data?.items.some((document) => isProcessing(document.processing_status))
      ? activeRefreshInterval
      : idleRefreshInterval,
  });

  useEffect(() => {
    setPage(1);
  }, [directoryId, keyword, sourceType, status]);

  useEffect(() => {
    if (query.data) onDocumentsChange?.(query.data.items, query.data.total);
  }, [onDocumentsChange, query.data]);

  if (query.isPending) return <LoadingState label="正在加载文档" />;
  if (query.error) return <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />;

  const documents = query.data?.items ?? [];
  if (documents.length === 0) {
    return <EmptyState title="暂无文档" description={keyword || status || sourceType ? '没有符合当前筛选条件的文档。' : '导入 Markdown、TXT、PDF、DOCX 和 URL。'} />;
  }

  return (
    <Box>
      <Box sx={{ display: 'grid', gridTemplateColumns: 'minmax(210px, 1.8fr) 70px 90px 105px 92px', alignItems: 'center', minHeight: 44, px: 2, bgcolor: '#fbfcff', borderBottom: '1px solid #edf0f5', color: '#71809a', fontSize: 12 }}>
        <span>文件名</span><span>类型</span><span>状态</span><span>更新时间</span><span>操作</span>
      </Box>
      {documents.map((document) => {
        const selected = selectedId === document.id;
        const extension = document.title.includes('.') ? document.title.split('.').pop()?.toUpperCase() : document.source_type === 'manual' ? 'MD' : 'FILE';
        return (
          <Box
            key={document.id}
            role="button"
            tabIndex={0}
            onClick={() => onSelect(document.id)}
            onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelect(document.id); } }}
            sx={{
              display: 'grid',
              gridTemplateColumns: 'minmax(210px, 1.8fr) 70px 90px 105px 92px',
              alignItems: 'center',
              minHeight: 54,
              px: 2,
              borderBottom: '1px solid #edf0f5',
              cursor: 'pointer',
              bgcolor: selected ? '#f0f2ff' : '#fff',
              color: selected ? '#4058e9' : '#172343',
              '&:hover': { bgcolor: selected ? '#f0f2ff' : '#fafbfe' },
              '&:focus-visible': { outline: '2px solid #6d7cff', outlineOffset: -2 },
            }}
          >
            <Stack direction="row" spacing={1.2} alignItems="center" minWidth={0}>
              <PictureAsPdfOutlined sx={{ fontSize: 20, color: extension === 'PDF' ? '#ef4444' : '#4e67f5' }} />
              <Typography noWrap sx={{ fontSize: 13, color: 'inherit' }}>{document.title}</Typography>
            </Stack>
            <Typography sx={{ fontSize: 12, color: '#65728a' }}>{extension}</Typography>
            <Chip
              size="small"
              label={isProcessing(document.processing_status) ? '处理中' : document.processing_status === 'failed' ? '失败' : '已完成'}
              sx={{ justifySelf: 'start', height: 24, borderRadius: 1.5, fontSize: 11, bgcolor: document.processing_status === 'failed' ? '#feecec' : isProcessing(document.processing_status) ? '#eaf2ff' : '#e8f7e9', color: document.processing_status === 'failed' ? '#d44b4b' : isProcessing(document.processing_status) ? '#4772d9' : '#32964a' }}
            />
            <Typography sx={{ fontSize: 12, color: '#65728a' }}>{new Date(document.updated_at).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })}</Typography>
            <Stack direction="row" spacing={0}>
              <Tooltip title="打开文档"><IconButton size="small" onClick={(event) => { event.stopPropagation(); onSelect(document.id); }}><VisibilityOutlined sx={{ fontSize: 17 }} /></IconButton></Tooltip>
              <Tooltip title={document.processing_status === 'failed' ? '重试处理' : '重新建立索引'}>
                <IconButton size="small" onClick={(event) => { event.stopPropagation(); if (document.processing_status === 'failed') onRetry(document); else onReindex(document); }} disabled={isProcessing(document.processing_status)}>
                  <CachedOutlined sx={{ fontSize: 17 }} />
                </IconButton>
              </Tooltip>
              <Tooltip title="删除文档"><IconButton size="small" onClick={(event) => { event.stopPropagation(); onDelete(document); }}><MoreVertOutlined sx={{ fontSize: 18 }} /></IconButton></Tooltip>
            </Stack>
          </Box>
        );
      })}
      <Stack direction="row" alignItems="center" sx={{ px: 2, py: 1.2, minHeight: 52 }}>
        <Typography sx={{ color: '#66728b', fontSize: 13, mr: 'auto' }}>共 {query.data?.total ?? documents.length} 条</Typography>
        <Pagination count={Math.max(1, Math.ceil((query.data?.total ?? documents.length) / pageSize))} page={page} onChange={(_, value) => setPage(value)} size="small" color="primary" />
        <Select value={pageSize} size="small" disabled sx={{ ml: 1, height: 34, fontSize: 12 }}><MenuItem value={pageSize}>{pageSize} 条/页</MenuItem></Select>
      </Stack>
    </Box>
  );
}

function ImportTasks({ kbId, onOpenDocument, embedded = false }: {
  kbId: string;
  onOpenDocument: (documentId: string) => void;
  embedded?: boolean;
}) {
  const queryClient = useQueryClient();
  const previousTaskStates = useRef<Map<string, string>>(new Map());
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [cleanupNotice, setCleanupNotice] = useState('');
  const [cleanupError, setCleanupError] = useState<Error | null>(null);
  const query = useQuery({
    queryKey: queryKeys.importTasks(kbId),
    queryFn: () => listImportTasks(kbId, { page: 1, page_size: 20 }),
    // 最近任务始终低频同步，避免任务在首次查询前快速完成后永久停留在旧缓存。
    refetchInterval: (result) => result.state.data?.items.some((task) => task.status === 'pending' || task.status === 'running')
      ? activeRefreshInterval
      : idleRefreshInterval,
  });
  const retry = useMutation({
    mutationFn: (taskId: string) => retryImportTask(taskId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBaseDashboard(kbId) });
    },
  });
  const start = useMutation({
    mutationFn: (taskId: string) => startImportTask(taskId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBaseDashboard(kbId) });
    },
  });
  const cleanup = useMutation({
    mutationFn: () => cleanupImportTasks(kbId),
    onSuccess: (result) => {
      setCleanupOpen(false);
      setCleanupError(null);
      setCleanupNotice(`已清理 ${result.deleted} 条已完成导入记录`);
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBaseDashboard(kbId) });
    },
    onError: (error) => setCleanupError(error as Error),
  });
  const completedCount = (query.data?.items ?? []).filter((task) => ['succeeded', 'failed', 'skipped'].includes(task.status)).length;

  useEffect(() => {
    const nextTaskStates = new Map<string, string>();
    let hasChanges = false;
    for (const task of query.data?.items ?? []) {
      const state = `${task.status}:${task.current_step ?? ''}:${task.document_id ?? ''}`;
      nextTaskStates.set(task.id, state);
      if (previousTaskStates.current.get(task.id) === state) continue;
      hasChanges = true;
      if (task.document_id) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.document(task.document_id) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.documentProcessing(task.document_id) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.documentContent(task.document_id) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.documentIndexVersions(task.document_id) });
      }
    }
    previousTaskStates.current = nextTaskStates;
    // Worker 创建文档、推进步骤或完成任务时，把列表和当前文档缓存一起同步。
    if (hasChanges) {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBaseDashboard(kbId) });
    }
  }, [kbId, query.dataUpdatedAt, query.data?.items, queryClient]);

  if (query.isPending) return <LoadingState label="正在加载导入任务" />;
  if (query.error) return <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />;
  const tasks = query.data?.items ?? [];

  return (
    <Paper variant={embedded ? undefined : 'outlined'} elevation={0} sx={{ p: 2.25, borderRadius: embedded ? 0 : 3, borderColor: '#e3e7ef', boxShadow: embedded ? 'none' : '0 6px 20px rgba(31,45,90,.025)' }}>
      <Stack direction="row" alignItems="center" mb={1.5}>
        <Typography sx={{ flexGrow: 1, color: '#182441', fontSize: 16, fontWeight: 650 }}>导入任务</Typography>
        {completedCount > 0 && (
          <Button size="small" disabled={cleanup.isPending} onClick={() => setCleanupOpen(true)}>
            {cleanup.isPending ? '正在清理…' : `清理已完成（${completedCount}）`}
          </Button>
        )}
        <Button size="small" startIcon={<RefreshOutlined />} onClick={() => void query.refetch()}>刷新</Button>
      </Stack>
      {cleanupNotice && <Alert severity="success" sx={{ mb: 1 }} onClose={() => setCleanupNotice('')}>{cleanupNotice}</Alert>}
      {cleanupError && <Alert severity="error" sx={{ mb: 1 }} onClose={() => setCleanupError(null)}>清理失败：{errorMessage(cleanupError)}</Alert>}
      {retry.error && <Alert severity="error" sx={{ mb: 1 }}>{errorMessage(retry.error)}</Alert>}
      {tasks.length === 0 && <EmptyState title="暂无导入任务" description="通过“导入知识”添加文件或 URL。" />}
      {tasks.length > 0 && (
        <>
      <Box sx={{ display: 'grid', gridTemplateColumns: 'minmax(220px, 1.7fr) 100px 80px 90px 150px 110px', alignItems: 'center', py: 1, px: 0.5, color: '#73809a', fontSize: 12, borderBottom: '1px solid #edf0f5' }}>
        <span>文件名</span><span>任务类型</span><span>文件数量</span><span>状态</span><span>完成时间</span><span>操作</span>
      </Box>
      <Stack spacing={0}>
        {tasks.map((task) => (
          <Box key={task.id} sx={{ display: 'grid', gridTemplateColumns: 'minmax(220px, 1.7fr) 100px 80px 90px 150px 110px', alignItems: 'center', minHeight: 46, px: 0.5, borderBottom: '1px solid #f0f2f6' }}>
            <Typography sx={{ fontSize: 13 }} noWrap>{task.source_path || task.file_name || task.source_url || task.id}</Typography>
            <Chip size="small" label={task.source_type === 'url' ? 'URL 导入' : '手动导入'} sx={{ justifySelf: 'start', height: 24, bgcolor: '#eeeaff', color: '#7257d8', fontSize: 11 }} />
            <Typography sx={{ fontSize: 12, color: '#65728a' }}>1</Typography>
            <Chip size="small" color={task.status === 'failed' ? 'error' : task.status === 'succeeded' ? 'success' : task.status === 'running' ? 'info' : 'default'} label={taskLabel[task.status]} sx={{ justifySelf: 'start', height: 24, fontSize: 11 }} />
            <Typography sx={{ fontSize: 12, color: '#65728a' }}>{new Date(task.completed_at || task.created_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</Typography>
            <Box>
              {task.document_id && <Button size="small" endIcon={<VisibilityOutlined sx={{ fontSize: 15 }} />} onClick={() => onOpenDocument(task.document_id as string)}>查看详情</Button>}
            </Box>
            {task.status === 'pending' && (
              <Tooltip title="开始解析（Markdown 可先补传图片）">
                <Button size="small" variant="outlined" disabled={start.isPending} onClick={() => start.mutate(task.id)}>
                  开始
                </Button>
              </Tooltip>
            )}
            {task.status === 'failed' && (
              <Tooltip title="重试导入"><IconButton size="small" disabled={retry.isPending} onClick={() => retry.mutate(task.id)}><RefreshOutlined fontSize="small" /></IconButton></Tooltip>
            )}
          </Box>
        ))}
      </Stack>
        </>
      )}
      <Dialog open={cleanupOpen} onClose={cleanup.isPending ? undefined : () => setCleanupOpen(false)}>
        <DialogTitle>清理导入记录</DialogTitle>
        <DialogContent>
          <Typography>
            将删除本知识库内 {completedCount} 条已结束（已完成/失败/已跳过）的导入任务记录，进行中的任务不受影响。文档本身不会被删除。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCleanupOpen(false)} disabled={cleanup.isPending}>取消</Button>
          <Button color="error" variant="contained" disabled={cleanup.isPending} onClick={() => cleanup.mutate()}>
            {cleanup.isPending ? '正在清理…' : '确认清理'}
          </Button>
        </DialogActions>
      </Dialog>
    </Paper>
  );
}

export function DocumentWorkspaceContent({ status, kbId, documentId }: {
  status: CapabilityStatus;
  kbId: string;
  documentId?: string;
}) {
  const [importOpen, setImportOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [workspaceTab, setWorkspaceTab] = useState<WorkspaceTab>('documents');
  const [directoryId, setDirectoryId] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(documentId ?? null);
  const [keyword, setKeyword] = useState('');
  const [processingStatus, setProcessingStatus] = useState<DocumentStatusFilter>('');
  const [sourceType, setSourceType] = useState<'' | DocumentSourceType>('');
  const [deleteTarget, setDeleteTarget] = useState<DocumentListItem | null>(null);
  const [notice, setNotice] = useState('');
  const [actionError, setActionError] = useState<Error | null>(null);
  const queryClient = useQueryClient();
  const enabled = status === 'available' && kbId !== '';

  const knowledgeBaseQuery = useQuery({
    queryKey: queryKeys.knowledgeBase(kbId),
    queryFn: () => getKnowledgeBase(kbId),
    enabled,
  });
  const dashboardQuery = useQuery({
    queryKey: queryKeys.knowledgeBaseDashboard(kbId),
    queryFn: () => getKnowledgeBaseDashboard(kbId),
    enabled,
    refetchInterval: (result) => (result.state.data?.processing_total ?? 0) > 0 ? 5000 : 30_000,
  });

  const treeQuery = useQuery({
    queryKey: queryKeys.directories(kbId),
    queryFn: () => getDirectoryTree(kbId),
    enabled,
  });
  const documentQuery = useQuery({
    queryKey: queryKeys.document(selectedId ?? ''),
    queryFn: () => getDocument(selectedId as string),
    enabled: enabled && Boolean(selectedId),
    refetchInterval: (result) => isProcessing(result.state.data?.processing_status)
      ? activeRefreshInterval
      : idleRefreshInterval,
  });
  const processingQuery = useQuery({
    queryKey: queryKeys.documentProcessing(selectedId ?? ''),
    queryFn: () => getDocumentProcessing(selectedId as string),
    enabled: enabled && Boolean(selectedId),
    refetchInterval: (result) => isProcessing(result.state.data?.processing_status)
      ? activeRefreshInterval
      : idleRefreshInterval,
  });

  const createDir = useMutation({
    mutationFn: ({ name, parentId }: { name: string; parentId?: string }) => createDirectory(kbId, { name, parent_id: parentId }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.directories(kbId) });
      setNotice('目录已创建');
    },
    onError: (error) => setActionError(error as Error),
  });
  const retryDoc = useMutation({
    mutationFn: retryDocumentProcessing,
    onSuccess: (_, documentIdValue) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.document(documentIdValue) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.documentProcessing(documentIdValue) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBaseDashboard(kbId) });
      setNotice('已提交处理重试');
    },
    onError: (error) => setActionError(error as Error),
  });
  const reindex = useMutation({
    mutationFn: reindexDocument,
    onSuccess: (_, documentIdValue) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.document(documentIdValue) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.documentProcessing(documentIdValue) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.documentContent(documentIdValue) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.documentIndexVersions(documentIdValue) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBaseDashboard(kbId) });
      setNotice('已提交重新索引');
    },
    onError: (error) => setActionError(error as Error),
  });
  const removeDoc = useMutation({
    mutationFn: deleteDocument,
    onSuccess: (_, documentIdValue) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.knowledgeBaseDashboard(kbId) });
      if (selectedId === documentIdValue) setSelectedId(null);
      setDeleteTarget(null);
      setNotice('文档已删除');
    },
    onError: (error) => setActionError(error as Error),
  });

  const directoryCount = treeQuery.data?.reduce((count, node) => {
    const countNode = (current: typeof node): number => 1 + current.children.reduce((sum, child) => sum + countNode(child), 0);
    return count + countNode(node);
  }, 0) ?? 0;
  const knowledgeBaseName = knowledgeBaseQuery.data?.name || '知识库';
  const dashboard = dashboardQuery.data;
  const healthLabel = (dashboard?.failed_total ?? 0) > 0
    ? '存在异常'
    : (dashboard?.processing_total ?? 0) > 0 ? '处理中' : '运行正常';
  const healthColor = (dashboard?.failed_total ?? 0) > 0 ? 'error' : (dashboard?.processing_total ?? 0) > 0 ? 'info' : 'success';

  return (
    <Stack spacing={1.7} sx={{ width: '100%', maxWidth: 1600, mx: 'auto' }}>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ color: '#71809a', fontSize: 13 }}>
        <Typography component={Link} to="/knowledge-bases" sx={{ color: '#71809a', textDecoration: 'none', fontSize: 13 }}>知识库</Typography>
        <span>/</span>
        <Typography sx={{ color: '#26324d', fontSize: 13 }}>{knowledgeBaseName}</Typography>
      </Stack>
      <Stack direction={{ xs: 'column', md: 'row' }} alignItems={{ md: 'center' }} gap={1.2}>
        <Box sx={{ flexGrow: 1 }}>
          <Stack direction="row" spacing={1.2} alignItems="center" flexWrap="wrap" useFlexGap>
            <Typography component="h1" sx={{ color: '#111c3a', fontSize: { xs: 26, md: 30 }, fontWeight: 750, lineHeight: 1.2 }}>{knowledgeBaseName}</Typography>
            <Chip size="small" color={healthColor} icon={healthColor === 'error' ? <ErrorOutlineRounded /> : healthColor === 'info' ? <PendingActionsOutlined /> : <CheckCircleOutlineRounded />} label={healthLabel} sx={{ height: 25, fontWeight: 650 }} />
          </Stack>
          <Typography sx={{ color: '#66728c', fontSize: 14, mt: 0.55 }}>{knowledgeBaseQuery.data?.description || '管理知识、数据源与检索质量。'}</Typography>
        </Box>
        <Button onClick={() => setSearchOpen(true)} startIcon={<SearchOutlined />} variant="outlined" disabled={!enabled} sx={{ borderRadius: 2.5, height: 42 }}>检索测试</Button>
        <Button onClick={() => setSettingsOpen(true)} startIcon={<SettingsOutlined />} variant="outlined" disabled={!enabled} sx={{ borderRadius: 2.5, height: 42 }}>知识库设置</Button>
        <Button startIcon={<UploadFileOutlined />} variant="contained" disabled={!enabled} onClick={() => setImportOpen(true)} sx={{ borderRadius: 2.5, height: 42, px: 2.2, background: 'linear-gradient(135deg, #3e65f3, #5546e8)', boxShadow: '0 8px 20px rgba(72,82,224,.2)' }}>导入知识</Button>
      </Stack>

      {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
      {actionError && <Alert severity="error" onClose={() => setActionError(null)}>操作失败：{errorMessage(actionError)}</Alert>}

      {!enabled && (
        <UnavailableState title="文档后端待接入" description="当前不会加载目录、文档或导入任务。" capability="document" />
      )}
      {enabled && dashboardQuery.isPending && <LoadingState label="正在加载知识库概览" />}
      {enabled && dashboardQuery.error && <Alert severity="warning" action={<Button color="inherit" size="small" onClick={() => void dashboardQuery.refetch()}>重试</Button>}>知识库概览加载失败：{errorMessage(dashboardQuery.error)}</Alert>}
      {enabled && dashboard && (
        <>
          <Paper variant="outlined" sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(3, minmax(0, 1fr))' }, borderRadius: 3, borderColor: '#e3e7ef', overflow: 'hidden' }}>
            <MetricItem label="文档总数" value={dashboard.document_total} detail={`${directoryCount} 个目录`} />
            <MetricItem label="处理成功" value={dashboard.indexed_total} detail="已完成处理" tone="success" />
            <MetricItem label="处理失败" value={dashboard.failed_total} detail={dashboard.failed_total > 0 ? '需要人工处理' : '当前无失败'} tone={dashboard.failed_total > 0 ? 'error' : 'default'} />
          </Paper>

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1.25fr) minmax(360px, .9fr)' }, gap: 1.5 }}>
            <ImportTrendChart data={dashboard.import_trend} />
            <RecentActivities data={dashboard.recent_activities} />
          </Box>

          <Paper variant="outlined" sx={{ borderRadius: 3, borderColor: '#e3e7ef', overflow: 'hidden', minHeight: 500 }}>
            <Tabs value={workspaceTab} onChange={(_, value: WorkspaceTab) => setWorkspaceTab(value)} sx={{ px: 1.5, minHeight: 48, borderBottom: '1px solid #e8ebf1', '& .MuiTab-root': { minHeight: 48, fontWeight: 650 } }}>
              <Tab value="documents" label={`文档库 ${dashboard.document_total}`} />
              <Tab value="imports" label="导入任务" />
            </Tabs>

            {workspaceTab === 'documents' && (
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', lg: '248px minmax(560px, 1fr)' }, alignItems: 'stretch' }}>
                <Box component="aside" sx={{ minHeight: 446, borderRight: { lg: '1px solid #e8ebf1' }, borderBottom: { xs: '1px solid #e8ebf1', lg: 0 } }}>
                  {treeQuery.isPending && <LoadingState label="正在加载目录" />}
                  {treeQuery.error && <ErrorState error={treeQuery.error as Error} onRetry={() => void treeQuery.refetch()} />}
                  {treeQuery.data && (
                    <KnowledgeTree
                      nodes={treeQuery.data}
                      selectedId={directoryId}
                      onSelect={setDirectoryId}
                      onCreateDirectory={(name, parentId) => createDir.mutate({ name, parentId })}
                      totalDocuments={dashboard.document_total}
                    />
                  )}
                </Box>
                <Box sx={{ minWidth: 0 }}>
                  <Stack direction={{ xs: 'column', md: 'row' }} spacing={1} p={1.3}>
                    <TextField
                      size="small"
                      placeholder="搜索文件名、内容或关键词..."
                      value={keyword}
                      onChange={(event) => setKeyword(event.target.value)}
                      InputProps={{ startAdornment: <InputAdornment position="start"><SearchOutlined fontSize="small" /></InputAdornment> }}
                      sx={{ flexGrow: 1, '& .MuiOutlinedInput-root': { height: 40, borderRadius: 2.2 } }}
                    />
                    <TextField select size="small" label="状态" value={processingStatus} onChange={(event) => setProcessingStatus(event.target.value as DocumentStatusFilter)} slotProps={{ inputLabel: { shrink: true }, select: { displayEmpty: true } }} sx={{ minWidth: 118, '& .MuiOutlinedInput-root': { height: 40, borderRadius: 2.2 } }}>
                      <MenuItem value="">全部状态</MenuItem>
                      {documentStatusOptions.map(({ value, label }) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
                    </TextField>
                    <TextField select size="small" label="来源" value={sourceType} onChange={(event) => setSourceType(event.target.value as '' | DocumentSourceType)} slotProps={{ inputLabel: { shrink: true }, select: { displayEmpty: true } }} sx={{ minWidth: 118, '& .MuiOutlinedInput-root': { height: 40, borderRadius: 2.2 } }}>
                      <MenuItem value="">全部来源</MenuItem>
                      {Object.entries(sourceLabel).map(([value, label]) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
                    </TextField>
                  </Stack>
                  <Box sx={{ overflowX: 'auto', borderTop: 1, borderColor: 'divider' }}>
                    <Box sx={{ minWidth: 640 }}>
                      <DocumentList
                        kbId={kbId}
                        directoryId={directoryId}
                        keyword={keyword}
                        status={processingStatus}
                        sourceType={sourceType}
                        selectedId={selectedId}
                        onSelect={setSelectedId}
                        onDelete={setDeleteTarget}
                        onRetry={(document) => retryDoc.mutate(document.id)}
                        onReindex={(document) => reindex.mutate(document.id)}
                      />
                    </Box>
                  </Box>
                </Box>
              </Box>
            )}

            {workspaceTab === 'imports' && <ImportTasks kbId={kbId} onOpenDocument={setSelectedId} embedded />}
          </Paper>
        </>
      )}

      <WorkspaceFeatureDialog
        open={selectedId !== null}
        onClose={() => setSelectedId(null)}
        icon={<DescriptionOutlined />}
        title={documentQuery.data?.title || '文档详情'}
        description="查看文档内容、来源与处理状态。"
      >
        {documentQuery.isPending && <LoadingState label="正在加载文档" />}
        {documentQuery.error && <ErrorState error={documentQuery.error as Error} onRetry={() => void documentQuery.refetch()} />}
        {processingQuery.error && <Alert severity="warning">处理状态加载失败：{errorMessage(processingQuery.error)}</Alert>}
        {documentQuery.data && <DocumentViewer document={documentQuery.data} processing={processingQuery.data} />}
      </WorkspaceFeatureDialog>
      <WorkspaceFeatureDialog
        open={searchOpen}
        onClose={() => setSearchOpen(false)}
        icon={<SearchOutlined />}
        title="检索测试"
        description="验证关键词、向量、融合排序与引用结果。"
      >
        <SearchTestPageContent status={capabilities.search} kbId={kbId} embedded />
      </WorkspaceFeatureDialog>
      <WorkspaceFeatureDialog
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        icon={<SettingsOutlined />}
        title="知识库设置"
        description="管理基础信息、默认模型与检索参数。"
      >
        <KnowledgeBaseSettingsContent status={capabilities.knowledgeBase} kbId={kbId} embedded />
      </WorkspaceFeatureDialog>
      <ImportDrawer open={importOpen} onClose={() => setImportOpen(false)} disabled={!enabled} kbId={kbId} directories={treeQuery.data ?? []} />
      <Dialog open={deleteTarget !== null} onClose={removeDoc.isPending ? undefined : () => setDeleteTarget(null)}>
        <DialogTitle>删除文档</DialogTitle>
        <DialogContent><Typography>确定删除“{deleteTarget?.title}”吗？文档将被软删除。</Typography></DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)} disabled={removeDoc.isPending}>取消</Button>
          <Button color="error" variant="contained" disabled={removeDoc.isPending} onClick={() => deleteTarget && removeDoc.mutate(deleteTarget.id)}>删除</Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}

export function DocumentWorkspacePage() {
  const { kbId = '', documentId } = useParams();
  return <DocumentWorkspaceContent status={capabilities.document} kbId={kbId} documentId={documentId} />;
}
