import CachedOutlined from '@mui/icons-material/CachedOutlined';
import CreateOutlined from '@mui/icons-material/CreateOutlined';
import DeleteOutlineOutlined from '@mui/icons-material/DeleteOutlineOutlined';
import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import SettingsOutlined from '@mui/icons-material/SettingsOutlined';
import UploadFileOutlined from '@mui/icons-material/UploadFileOutlined';
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
  List,
  ListItemButton,
  ListItemText,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { DocumentWorkspace } from '@/layouts/DocumentWorkspace';
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
  documentStatusLabel,
  documentStatusOptions,
  type DocumentStatusFilter,
} from '../status';
import type {
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

function isProcessing(status?: DocumentProcessingStatus) {
  return status !== undefined && activeProcessingStatuses.includes(status);
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
}) {
  const query = useQuery({
    queryKey: [...queryKeys.documents(kbId), { directoryId, keyword, status, sourceType }],
    queryFn: () => listDocuments(kbId, {
      page: 1,
      page_size: 100,
      directory_id: directoryId || undefined,
      keyword: keyword.trim() || undefined,
      ...documentStatusFilterParams(status),
      source_type: sourceType || undefined,
    }),
    // 仅存在处理中记录时轮询；进入终态后自动停止，避免无意义请求。
    refetchInterval: (result) => result.state.data?.items.some((document) => isProcessing(document.processing_status)) ? 3000 : false,
  });

  if (query.isPending) return <LoadingState label="正在加载文档" />;
  if (query.error) return <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />;

  const documents = query.data?.items ?? [];
  if (documents.length === 0) {
    return <EmptyState title="暂无文档" description={keyword || status || sourceType ? '没有符合当前筛选条件的文档。' : '新建手工文档，或导入 Markdown、TXT、PDF、DOCX 和 URL。'} />;
  }

  return (
    <List disablePadding dense>
      {documents.map((document) => (
        <ListItemButton key={document.id} selected={selectedId === document.id} onClick={() => onSelect(document.id)}>
          <ListItemText
            primary={document.title}
            secondary={`${documentStatusLabel(document.processing_status, document.index_mode)} · ${sourceLabel[document.source_type]}`}
          />
          {isProcessing(document.processing_status) && <Chip size="small" color="info" label="处理中" sx={{ mr: 0.5 }} />}
          <Tooltip title={document.processing_status === 'failed' ? '重试处理' : '仅失败文档可重试'}>
            <span>
              <IconButton size="small" disabled={document.processing_status !== 'failed'} onClick={(event) => { event.stopPropagation(); onRetry(document); }}>
                <RefreshOutlined fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title={document.processing_status === 'succeeded' ? '重新建立索引' : '处理完成后可重新索引'}>
            <span>
              <IconButton size="small" disabled={document.processing_status !== 'succeeded'} onClick={(event) => { event.stopPropagation(); onReindex(document); }}>
                <CachedOutlined fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title="删除文档">
            <IconButton size="small" color="error" onClick={(event) => { event.stopPropagation(); onDelete(document); }}>
              <DeleteOutlineOutlined fontSize="small" />
            </IconButton>
          </Tooltip>
        </ListItemButton>
      ))}
    </List>
  );
}

function ImportTasks({ kbId, onOpenDocument }: { kbId: string; onOpenDocument: (documentId: string) => void }) {
  const queryClient = useQueryClient();
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [cleanupNotice, setCleanupNotice] = useState('');
  const [cleanupError, setCleanupError] = useState<Error | null>(null);
  const query = useQuery({
    queryKey: queryKeys.importTasks(kbId),
    queryFn: () => listImportTasks(kbId, { page: 1, page_size: 20 }),
    // pending/running 期间每三秒刷新一次，成功、失败或跳过后停止轮询。
    refetchInterval: (result) => result.state.data?.items.some((task) => task.status === 'pending' || task.status === 'running') ? 3000 : false,
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
    // Worker 创建出文档或完成任务后，主动刷新文档列表以展示最新结果。
    if (query.data?.items.some((task) => task.document_id || task.status === 'succeeded')) {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
    }
  }, [kbId, query.dataUpdatedAt, query.data?.items, queryClient]);

  if (query.isPending) return <LoadingState label="正在加载导入任务" />;
  if (query.error) return <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />;
  const tasks = query.data?.items ?? [];
  if (tasks.length === 0) return null;

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack direction="row" alignItems="center" mb={1}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ flexGrow: 1 }}>最近导入任务</Typography>
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
      <Stack spacing={1}>
        {tasks.map((task) => (
          <Stack key={task.id} direction="row" alignItems="center" spacing={1}>
            <Box sx={{ flexGrow: 1, minWidth: 0 }}>
              <Typography variant="body2" noWrap>{task.file_name || task.source_url || task.id}</Typography>
              <Typography variant="caption" color={task.failure_reason ? 'error' : 'text.secondary'} noWrap display="block">
                {task.failure_reason || task.current_step || new Date(task.created_at).toLocaleString()}
              </Typography>
            </Box>
            <Chip size="small" color={task.status === 'failed' ? 'error' : task.status === 'succeeded' ? 'success' : task.status === 'running' ? 'info' : 'default'} label={taskLabel[task.status]} />
            {task.document_id && <Button size="small" onClick={() => onOpenDocument(task.document_id as string)}>查看文档</Button>}
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
          </Stack>
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
  const queryClient = useQueryClient();
  const enabled = status === 'available' && kbId !== '';

  const treeQuery = useQuery({
    queryKey: queryKeys.directories(kbId),
    queryFn: () => getDirectoryTree(kbId),
    enabled,
  });
  const documentQuery = useQuery({
    queryKey: queryKeys.document(selectedId ?? ''),
    queryFn: () => getDocument(selectedId as string),
    enabled: enabled && Boolean(selectedId),
    // 文档详情同样只在处理中轮询，避免页面常驻时持续访问后端。
    refetchInterval: (result) => isProcessing(result.state.data?.processing_status) ? 3000 : false,
  });
  const processingQuery = useQuery({
    queryKey: queryKeys.documentProcessing(selectedId ?? ''),
    queryFn: () => getDocumentProcessing(selectedId as string),
    enabled: enabled && Boolean(selectedId),
    refetchInterval: (result) => isProcessing(result.state.data?.processing_status) ? 3000 : false,
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

  return (
    <Stack spacing={2}>
      <Stack direction={{ xs: 'column', sm: 'row' }} alignItems={{ sm: 'center' }} gap={1}>
        <Box sx={{ flexGrow: 1 }}>
          <Typography component="h2" variant="h5" fontWeight={750}>文档工作区</Typography>
          <Typography color="text.secondary">管理目录、手工文档、文件导入任务与索引处理状态。</Typography>
        </Box>
        <Button component={Link} to={`/kb/${kbId}/search-test`} startIcon={<SearchOutlined />} variant="outlined" disabled={!enabled}>检索测试</Button>
        <Button component={Link} to={`/kb/${kbId}/settings`} startIcon={<SettingsOutlined />} variant="outlined" disabled={!enabled}>知识库设置</Button>
        <Button startIcon={<CreateOutlined />} variant="outlined" disabled={!enabled} onClick={() => setManualOpen(true)}>手工新建</Button>
        <Button startIcon={<UploadFileOutlined />} variant="contained" disabled={!enabled} onClick={() => setImportOpen(true)}>导入知识</Button>
      </Stack>

      {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
      {actionError && <Alert severity="error" onClose={() => setActionError(null)}>操作失败：{errorMessage(actionError)}</Alert>}

      {!enabled && (
        <UnavailableState title="文档后端待接入" description="当前不会加载目录、文档或导入任务。" capability="document" />
      )}
      {enabled && treeQuery.isPending && <LoadingState label="正在加载目录" />}
      {enabled && treeQuery.error && <ErrorState error={treeQuery.error as Error} onRetry={() => void treeQuery.refetch()} />}
      {enabled && treeQuery.data && (
        <DocumentWorkspace
          sidebar={
            <KnowledgeTree
              nodes={treeQuery.data}
              selectedId={directoryId}
              onSelect={setDirectoryId}
              onCreateDirectory={(name, parentId) => createDir.mutate({ name, parentId })}
            />
          }
        >
          <Stack spacing={2}>
            <Paper variant="outlined" sx={{ overflow: 'hidden' }}>
              <Stack direction={{ xs: 'column', md: 'row' }} spacing={1} p={1.5}>
                <TextField
                  size="small"
                  placeholder="按标题搜索"
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  InputProps={{ startAdornment: <InputAdornment position="start"><SearchOutlined fontSize="small" /></InputAdornment> }}
                  sx={{ flexGrow: 1 }}
                />
                <TextField select size="small" label="状态" value={processingStatus} onChange={(event) => setProcessingStatus(event.target.value as DocumentStatusFilter)} sx={{ minWidth: 190 }}>
                  <MenuItem value="">全部状态</MenuItem>
                  {documentStatusOptions.map(({ value, label }) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
                </TextField>
                <TextField select size="small" label="来源" value={sourceType} onChange={(event) => setSourceType(event.target.value as '' | DocumentSourceType)} sx={{ minWidth: 130 }}>
                  <MenuItem value="">全部来源</MenuItem>
                  {Object.entries(sourceLabel).map(([value, label]) => <MenuItem key={value} value={value}>{label}</MenuItem>)}
                </TextField>
              </Stack>
              <Box sx={{ maxHeight: 360, overflow: 'auto', borderTop: 1, borderColor: 'divider' }}>
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
            </Paper>
            <ImportTasks kbId={kbId} onOpenDocument={setSelectedId} />
            {!selectedId && <EmptyState title="选择一篇文档" description="从上方文档列表选择文档，查看详情与索引状态。" />}
            {selectedId && (documentQuery.isPending || processingQuery.isPending) && <LoadingState label="正在加载文档" />}
            {selectedId && documentQuery.error && <ErrorState error={documentQuery.error as Error} onRetry={() => void documentQuery.refetch()} />}
            {selectedId && processingQuery.error && <ErrorState error={processingQuery.error as Error} onRetry={() => void processingQuery.refetch()} />}
            {documentQuery.data && <DocumentViewer document={documentQuery.data} processing={processingQuery.data} />}
          </Stack>
        </DocumentWorkspace>
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
