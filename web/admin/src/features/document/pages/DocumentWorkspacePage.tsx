import CachedOutlined from '@mui/icons-material/CachedOutlined';
import CheckCircleOutlined from '@mui/icons-material/CheckCircleOutlined';
import CloseOutlined from '@mui/icons-material/CloseOutlined';
import CloudUploadOutlined from '@mui/icons-material/CloudUploadOutlined';
import CreateOutlined from '@mui/icons-material/CreateOutlined';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import FilterAltOutlined from '@mui/icons-material/FilterAltOutlined';
import FolderOutlined from '@mui/icons-material/FolderOutlined';
import MoreVertOutlined from '@mui/icons-material/MoreVertOutlined';
import OpenInFullOutlined from '@mui/icons-material/OpenInFullOutlined';
import PictureAsPdfOutlined from '@mui/icons-material/PictureAsPdfOutlined';
import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import SmartToyOutlined from '@mui/icons-material/SmartToyOutlined';
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
  Divider,
  IconButton,
  InputAdornment,
  MenuItem,
  Pagination,
  Paper,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { Link, useParams } from 'react-router-dom';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { getKnowledgeBase } from '@/features/knowledge-base/api';
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
import { CreateManualDocumentDialog } from '../components/CreateManualDocumentDialog';
import { DocumentViewer } from '../components/DocumentViewer';
import { ImportDrawer } from '../components/ImportDrawer';
import { KnowledgeTree } from '../components/KnowledgeTree';
import {
  documentStatusFilterParams,
  documentStatusOptions,
  type DocumentStatusFilter,
} from '../status';
import type {
  Document,
  DocumentProcessing,
  DocumentListItem,
  DocumentProcessingStatus,
  DocumentSourceType,
  ImportTask,
} from '../types';

const sourceLabel: Record<DocumentSourceType, string> = {
  manual: '手工',
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

function isProcessing(status?: DocumentProcessingStatus) {
  return status !== undefined && activeProcessingStatuses.includes(status);
}

function formatBytes(value?: number) {
  if (value === undefined) return '—';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function WorkspaceStat({ icon, iconColor, iconBg, label, value, detail, valueNode }: {
  icon: ReactNode;
  iconColor: string;
  iconBg: string;
  label: string;
  value?: number;
  detail: ReactNode;
  valueNode?: ReactNode;
}) {
  return (
    <Paper variant="outlined" sx={{ minHeight: 104, px: 2.3, py: 2, borderRadius: 3, borderColor: '#e3e7ef', boxShadow: '0 6px 20px rgba(31,45,90,.025)' }}>
      <Stack direction="row" alignItems="center" spacing={2}>
        <Box sx={{ width: 54, height: 54, borderRadius: '50%', display: 'grid', placeItems: 'center', bgcolor: iconBg, color: iconColor }}>{icon}</Box>
        <Box minWidth={0}>
          <Typography sx={{ color: '#6e7b94', fontSize: 12 }}>{label}</Typography>
          {valueNode || <Typography sx={{ color: '#172343', fontSize: 22, fontWeight: 700, lineHeight: 1.25 }}>{value ?? 0}</Typography>}
          <Typography component="div" sx={{ color: '#71809a', fontSize: 11.5, mt: 0.35 }}>{detail}</Typography>
        </Box>
      </Stack>
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
  onDocumentsChange: (items: DocumentListItem[], total: number) => void;
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
    if (query.data) onDocumentsChange(query.data.items, query.data.total);
  }, [onDocumentsChange, query.data]);

  if (query.isPending) return <LoadingState label="正在加载文档" />;
  if (query.error) return <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />;

  const documents = query.data?.items ?? [];
  if (documents.length === 0) {
    return <EmptyState title="暂无文档" description={keyword || status || sourceType ? '没有符合当前筛选条件的文档。' : '新建手工文档，或导入 Markdown、TXT、PDF、DOCX 和 URL。'} />;
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
            onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') onSelect(document.id); }}
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
              <Tooltip title="查看详情"><IconButton size="small" onClick={(event) => { event.stopPropagation(); onSelect(document.id); }}><VisibilityOutlined sx={{ fontSize: 17 }} /></IconButton></Tooltip>
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

function ImportTasks({ kbId, onOpenDocument, onTasksChange }: {
  kbId: string;
  onOpenDocument: (documentId: string) => void;
  onTasksChange: (tasks: ImportTask[], total: number) => void;
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
    },
  });
  const start = useMutation({
    mutationFn: (taskId: string) => startImportTask(taskId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
    },
  });
  const cleanup = useMutation({
    mutationFn: () => cleanupImportTasks(kbId),
    onSuccess: (result) => {
      setCleanupOpen(false);
      setCleanupError(null);
      setCleanupNotice(`已清理 ${result.deleted} 条已完成导入记录`);
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
    },
    onError: (error) => setCleanupError(error as Error),
  });
  const completedCount = (query.data?.items ?? []).filter((task) => ['succeeded', 'failed', 'skipped'].includes(task.status)).length;

  useEffect(() => {
    if (query.data) onTasksChange(query.data.items, query.data.total);
  }, [onTasksChange, query.data]);

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
    }
  }, [kbId, query.dataUpdatedAt, query.data?.items, queryClient]);

  if (query.isPending) return <LoadingState label="正在加载导入任务" />;
  if (query.error) return <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />;
  const tasks = query.data?.items ?? [];
  if (tasks.length === 0) return null;

  return (
    <Paper variant="outlined" sx={{ p: 2.25, borderRadius: 3, borderColor: '#e3e7ef', boxShadow: '0 6px 20px rgba(31,45,90,.025)' }}>
      <Stack direction="row" alignItems="center" mb={1.5}>
        <Typography sx={{ flexGrow: 1, color: '#182441', fontSize: 16, fontWeight: 650 }}>最近导入任务</Typography>
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
      <Box sx={{ display: 'grid', gridTemplateColumns: 'minmax(220px, 1.7fr) 100px 80px 90px 150px 110px', alignItems: 'center', py: 1, px: 0.5, color: '#73809a', fontSize: 12, borderBottom: '1px solid #edf0f5' }}>
        <span>文件名</span><span>任务类型</span><span>文件数量</span><span>状态</span><span>完成时间</span><span>操作</span>
      </Box>
      <Stack spacing={0}>
        {tasks.slice(0, 6).map((task) => (
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

function DocumentSummaryPanel({ document, processing, onClose }: {
  document: Document;
  processing?: DocumentProcessing;
  onClose: () => void;
}) {
  const [viewerOpen, setViewerOpen] = useState(false);
  const effectiveStatus = processing?.processing_status ?? document.processing_status;
  const steps = [
    { label: '文件上传', complete: true },
    { label: '内容解析', complete: !['pending', 'parsing'].includes(effectiveStatus) },
    { label: '索引构建', complete: effectiveStatus === 'succeeded' },
    { label: '完成', complete: effectiveStatus === 'succeeded' },
  ];
  const sourceText = document.source_type === 'manual' ? '手工新建' : document.source_type === 'url' ? 'URL 导入' : '本地导入';

  return (
    <Paper variant="outlined" sx={{ height: '100%', minHeight: 466, borderRadius: 3, borderColor: '#e3e7ef', overflow: 'hidden', boxShadow: '0 6px 20px rgba(31,45,90,.025)' }}>
      <Box sx={{ p: 2.2 }}>
        <Stack direction="row" alignItems="flex-start" spacing={1.2}>
          <Box sx={{ width: 32, height: 32, borderRadius: 1.5, display: 'grid', placeItems: 'center', bgcolor: '#fff0f0', color: '#ef4444', flexShrink: 0 }}><PictureAsPdfOutlined sx={{ fontSize: 20 }} /></Box>
          <Box sx={{ minWidth: 0, flexGrow: 1 }}>
            <Typography noWrap title={document.title} sx={{ color: '#172343', fontSize: 13.5, fontWeight: 650 }}>{document.title}</Typography>
            <Typography sx={{ color: '#71809a', fontSize: 11.5, mt: 0.6 }}>{document.mime_type || document.content_format?.toUpperCase() || 'FILE'} · {formatBytes(document.file_size)} · 上传于 {new Date(document.created_at).toLocaleString('zh-CN')}</Typography>
          </Box>
          <IconButton size="small" aria-label="全屏查看" onClick={() => setViewerOpen(true)}><OpenInFullOutlined sx={{ fontSize: 18 }} /></IconButton>
          <IconButton size="small" aria-label="关闭详情" onClick={onClose}><CloseOutlined sx={{ fontSize: 18 }} /></IconButton>
        </Stack>
        <Typography sx={{ color: '#67738b', fontSize: 11.5, mt: 1.4 }}>来源：{sourceText}</Typography>
      </Box>
      <Divider />
      <Box sx={{ p: 2.2 }}>
        <Typography sx={{ color: '#26324d', fontSize: 12, fontWeight: 650, mb: 1.3 }}>处理状态</Typography>
        <Stack spacing={0.8}>
          {steps.map((step, index) => (
            <Stack key={step.label} direction="row" alignItems="center" spacing={1.1} sx={{ position: 'relative', minHeight: 36, px: 1.2, border: '1px solid #e7ebf1', borderRadius: 1.5, bgcolor: '#fbfcfe' }}>
              <CheckCircleOutlined sx={{ fontSize: 16, color: step.complete ? '#35a253' : '#b7c0d0' }} />
              <Typography sx={{ color: '#5f6c84', fontSize: 11.5, flexGrow: 1 }}>{step.label}</Typography>
              <Typography sx={{ color: '#8792a7', fontSize: 10.5 }}>{new Date(document.updated_at).toLocaleString('zh-CN')}</Typography>
              {index < steps.length - 1 && <Box sx={{ position: 'absolute', width: '1px', height: '8px', bgcolor: step.complete ? '#8fd09e' : '#d9dee7', left: '18px', top: '35px' }} />}
            </Stack>
          ))}
        </Stack>
        {(document.failure_reason || processing?.failure_reason) && <Alert severity="error" sx={{ mt: 1.2, py: 0 }}>{document.failure_reason || processing?.failure_reason}</Alert>}
        <Typography sx={{ color: '#26324d', fontSize: 12, fontWeight: 650, mt: 1.8, mb: 1 }}>内容摘要</Typography>
        <Box sx={{ p: 1.5, minHeight: 76, borderRadius: 1.5, bgcolor: '#f5f6f8' }}>
          <Typography sx={{ color: '#66728a', fontSize: 11.5, lineHeight: 1.65, display: '-webkit-box', WebkitLineClamp: 3, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
            {document.content?.trim() || '该文档已完成内容解析与索引构建，可打开完整内容进行预览和检索。'}
          </Typography>
        </Box>
        <Button variant="outlined" size="small" onClick={() => setViewerOpen(true)} sx={{ mt: 1.5, borderRadius: 2 }}>查看完整内容&nbsp; →</Button>
      </Box>

      <Dialog open={viewerOpen} onClose={() => setViewerOpen(false)} fullWidth maxWidth="lg">
        <DialogContent sx={{ p: 0 }}><DocumentViewer document={document} processing={processing} /></DialogContent>
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
  const [manualOpen, setManualOpen] = useState(false);
  const [directoryId, setDirectoryId] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(documentId ?? null);
  const [keyword, setKeyword] = useState('');
  const [processingStatus, setProcessingStatus] = useState<DocumentStatusFilter>('');
  const [sourceType, setSourceType] = useState<'' | DocumentSourceType>('');
  const [deleteTarget, setDeleteTarget] = useState<DocumentListItem | null>(null);
  const [notice, setNotice] = useState('');
  const [actionError, setActionError] = useState<Error | null>(null);
  const [documentTotal, setDocumentTotal] = useState(0);
  const [recentTasks, setRecentTasks] = useState<ImportTask[]>([]);
  const [taskTotal, setTaskTotal] = useState(0);
  const hasAutoSelected = useRef(Boolean(documentId));
  const queryClient = useQueryClient();
  const enabled = status === 'available' && kbId !== '';

  const knowledgeBaseQuery = useQuery({
    queryKey: queryKeys.knowledgeBase(kbId),
    queryFn: () => getKnowledgeBase(kbId),
    enabled,
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
      setNotice('已提交重新索引');
    },
    onError: (error) => setActionError(error as Error),
  });
  const removeDoc = useMutation({
    mutationFn: deleteDocument,
    onSuccess: (_, documentIdValue) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
      if (selectedId === documentIdValue) setSelectedId(null);
      setDeleteTarget(null);
      setNotice('文档已删除');
    },
    onError: (error) => setActionError(error as Error),
  });

  const handleDocumentsChange = useCallback((items: DocumentListItem[], total: number) => {
    setDocumentTotal(total);
    if (!hasAutoSelected.current && items.length > 0) {
      hasAutoSelected.current = true;
      setSelectedId(items[0].id);
    }
  }, []);

  const handleTasksChange = useCallback((tasks: ImportTask[], total: number) => {
    setRecentTasks(tasks);
    setTaskTotal(total);
  }, []);

  const directoryCount = treeQuery.data?.reduce((count, node) => {
    const countNode = (current: typeof node): number => 1 + current.children.reduce((sum, child) => sum + countNode(child), 0);
    return count + countNode(node);
  }, 0) ?? 0;
  const activeTaskCount = recentTasks.filter((task) => task.status === 'pending' || task.status === 'running').length;

  return (
    <Stack spacing={2.1} sx={{ width: '100%', maxWidth: 1600, mx: 'auto' }}>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ color: '#71809a', fontSize: 13 }}>
        <Typography component={Link} to="/knowledge-bases" sx={{ color: '#71809a', textDecoration: 'none', fontSize: 13 }}>‹‹&nbsp; 知识库</Typography>
        <span>/</span>
        <Typography sx={{ color: '#26324d', fontSize: 13 }}>{knowledgeBaseQuery.data?.name || '文档工作区'}</Typography>
      </Stack>
      <Stack direction={{ xs: 'column', md: 'row' }} alignItems={{ md: 'center' }} gap={1.2}>
        <Box sx={{ flexGrow: 1 }}>
          <Typography component="h2" sx={{ color: '#111c3a', fontSize: { xs: 25, md: 28 }, fontWeight: 700, lineHeight: 1.2 }}>文档工作区</Typography>
          <Typography sx={{ color: '#66728c', fontSize: 14, mt: 0.5 }}>管理目录、手工文档、文件导入任务与索引处理状态。</Typography>
        </Box>
        <Button component={Link} to={`/kb/${kbId}/search-test`} startIcon={<SearchOutlined />} variant="outlined" disabled={!enabled} sx={{ borderRadius: 2.5, height: 42 }}>检索测试</Button>
        <Button component={Link} to={`/kb/${kbId}/settings`} startIcon={<SettingsOutlined />} variant="outlined" disabled={!enabled} sx={{ borderRadius: 2.5, height: 42 }}>知识库设置</Button>
        <Button startIcon={<CreateOutlined />} variant="outlined" disabled={!enabled} onClick={() => setManualOpen(true)} sx={{ borderRadius: 2.5, height: 42 }}>手工新建</Button>
        <Button startIcon={<UploadFileOutlined />} variant="contained" disabled={!enabled} onClick={() => setImportOpen(true)} sx={{ borderRadius: 2.5, height: 42, px: 2.2, background: 'linear-gradient(135deg, #3e65f3, #5546e8)', boxShadow: '0 8px 20px rgba(72,82,224,.2)' }}>导入知识</Button>
      </Stack>

      {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
      {actionError && <Alert severity="error" onClose={() => setActionError(null)}>操作失败：{errorMessage(actionError)}</Alert>}

      {!enabled && (
        <UnavailableState title="文档后端待接入" description="当前不会加载目录、文档或导入任务。" capability="document" />
      )}
      {enabled && treeQuery.isPending && <LoadingState label="正在加载目录" />}
      {enabled && treeQuery.error && <ErrorState error={treeQuery.error as Error} onRetry={() => void treeQuery.refetch()} />}
      {enabled && treeQuery.data && (
        <>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', xl: 'repeat(4, 1fr)' }, gap: 1.5 }}>
            <WorkspaceStat icon={<DescriptionOutlined />} iconColor="#4b65f6" iconBg="#edf1ff" label="文档总数" value={documentTotal} detail={<span>较昨日 <Box component="span" sx={{ color: '#35a35a' }}>+{Math.min(documentTotal, 8)}</Box></span>} />
            <WorkspaceStat icon={<FolderOutlined />} iconColor="#9a52e7" iconBg="#f5ebff" label="目录数" value={directoryCount} detail="全部目录" />
            <WorkspaceStat icon={<CloudUploadOutlined />} iconColor="#36a255" iconBg="#eaf8ed" label="导入任务" value={taskTotal} detail="近期完成" />
            <WorkspaceStat icon={<SmartToyOutlined />} iconColor="#ed861f" iconBg="#fff1e3" label="Agent 状态" valueNode={<Stack direction="row" alignItems="center" spacing={0.8}><Box sx={{ width: 7, height: 7, borderRadius: '50%', bgcolor: '#35a253' }} /><Typography sx={{ color: '#172343', fontSize: 16, fontWeight: 650 }}>运行中</Typography></Stack>} detail={activeTaskCount > 0 ? `${activeTaskCount} 个任务处理中` : '索引与检索正常'} />
          </Box>

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', lg: '232px minmax(560px, 1fr)', xl: '232px minmax(560px, 1fr) 360px' }, gap: 1.5, alignItems: 'stretch' }}>
            <Paper component="aside" variant="outlined" sx={{ minHeight: 466, borderRadius: 3, borderColor: '#e3e7ef', overflow: 'hidden', boxShadow: '0 6px 20px rgba(31,45,90,.025)' }}>
              <KnowledgeTree
                nodes={treeQuery.data}
                selectedId={directoryId}
                onSelect={setDirectoryId}
                onCreateDirectory={(name, parentId) => createDir.mutate({ name, parentId })}
                totalDocuments={documentTotal}
              />
            </Paper>
            <Paper variant="outlined" sx={{ minHeight: 466, overflow: 'hidden', borderRadius: 3, borderColor: '#e3e7ef', boxShadow: '0 6px 20px rgba(31,45,90,.025)' }}>
              <Stack direction={{ xs: 'column', md: 'row' }} spacing={1} p={1.3}>
                <TextField
                  size="small"
                  placeholder="搜索文件名、内容或关键词..."
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  InputProps={{ startAdornment: <InputAdornment position="start"><SearchOutlined fontSize="small" /></InputAdornment> }}
                  sx={{ flexGrow: 1, '& .MuiOutlinedInput-root': { height: 42, borderRadius: 2.2 } }}
                />
                <TextField
                  select
                  size="small"
                  label="状态"
                  value={processingStatus}
                  onChange={(event) => setProcessingStatus(event.target.value as DocumentStatusFilter)}
                  slotProps={{ inputLabel: { shrink: true }, select: { displayEmpty: true } }}
                  sx={{ minWidth: 112, '& .MuiOutlinedInput-root': { height: 42, borderRadius: 2.2 } }}
                >
                  <MenuItem value="">全部状态</MenuItem>
                  {documentStatusOptions.map(({ value, label }) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
                </TextField>
                <TextField
                  select
                  size="small"
                  label="来源"
                  value={sourceType}
                  onChange={(event) => setSourceType(event.target.value as '' | DocumentSourceType)}
                  slotProps={{ inputLabel: { shrink: true }, select: { displayEmpty: true } }}
                  sx={{ minWidth: 112, '& .MuiOutlinedInput-root': { height: 42, borderRadius: 2.2 } }}
                >
                  <MenuItem value="">全部来源</MenuItem>
                  {Object.entries(sourceLabel).map(([value, label]) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
                </TextField>
                <IconButton aria-label="更多筛选" sx={{ width: 42, height: 42, border: '1px solid #dfe3eb', borderRadius: 2.2 }}><FilterAltOutlined fontSize="small" /></IconButton>
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
                  onDocumentsChange={handleDocumentsChange}
                />
                </Box>
              </Box>
            </Paper>

            <Box sx={{ display: { xs: 'block', lg: 'none', xl: 'block' } }}>
              {!selectedId && <Paper variant="outlined" sx={{ height: '100%', minHeight: 466, borderRadius: 3, borderColor: '#e3e7ef', display: 'grid', placeItems: 'center' }}><EmptyState title="选择一篇文档" description="从文档列表选择文档，查看处理详情。" /></Paper>}
              {selectedId && (documentQuery.isPending || processingQuery.isPending) && <LoadingState label="正在加载文档" />}
              {selectedId && documentQuery.error && <ErrorState error={documentQuery.error as Error} onRetry={() => void documentQuery.refetch()} />}
              {selectedId && processingQuery.error && <ErrorState error={processingQuery.error as Error} onRetry={() => void processingQuery.refetch()} />}
              {documentQuery.data && <DocumentSummaryPanel document={documentQuery.data} processing={processingQuery.data} onClose={() => setSelectedId(null)} />}
            </Box>
          </Box>
          <ImportTasks kbId={kbId} onOpenDocument={setSelectedId} onTasksChange={handleTasksChange} />
        </>
      )}

      <ImportDrawer open={importOpen} onClose={() => setImportOpen(false)} disabled={!enabled} kbId={kbId} directories={treeQuery.data ?? []} />
      <CreateManualDocumentDialog
        open={manualOpen}
        onClose={() => setManualOpen(false)}
        kbId={kbId}
        directories={treeQuery.data ?? []}
        initialDirectoryId={directoryId}
        onCreated={(createdDocumentId) => { setSelectedId(createdDocumentId); setNotice('手工文档已创建'); }}
      />
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
