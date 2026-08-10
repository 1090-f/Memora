import CachedOutlined from '@mui/icons-material/CachedOutlined';
import DeleteOutlineOutlined from '@mui/icons-material/DeleteOutlineOutlined';
import RefreshOutlined from '@mui/icons-material/RefreshOutlined';
import UploadFileOutlined from '@mui/icons-material/UploadFileOutlined';
import { Alert, Box, Button, Chip, IconButton, List, ListItemButton, ListItemText, Paper, Stack, Tooltip, Typography } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { EmptyState } from '@/components/shared/EmptyState';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { DocumentWorkspace } from '@/layouts/DocumentWorkspace';
import { createDirectory, deleteDocument, getDirectoryTree, getDocument, listDocuments, listImportTasks, reindexDocument, retryDocumentProcessing, retryImportTask } from '../api';
import { DocumentViewer } from '../components/DocumentViewer';
import { ImportDrawer } from '../components/ImportDrawer';
import { KnowledgeTree } from '../components/KnowledgeTree';
import type { DocumentListItem, DocumentProcessingStatus, ImportTask } from '../types';

const processingLabel: Record<DocumentProcessingStatus, string> = {
  pending: '待处理',
  parsing: '解析中',
  cleaning: '清洗中',
  chunking: '分段中',
  embedding: '向量化中',
  keyword_indexing: '关键词索引中',
  succeeded: '已完成',
  failed: '失败',
};

const taskLabel: Record<ImportTask['status'], string> = {
  pending: '等待中',
  running: '进行中',
  succeeded: '已完成',
  failed: '失败',
  skipped: '已跳过',
};

function DocumentList({ kbId, directoryId, selectedId, onSelect, onDelete, onRetry, onReindex }: {
  kbId: string;
  directoryId: string | null;
  selectedId: string | null;
  onSelect: (id: string) => void;
  onDelete: (document: DocumentListItem) => void;
  onRetry: (document: DocumentListItem) => void;
  onReindex: (document: DocumentListItem) => void;
}) {
  const query = useQuery({
    queryKey: [...queryKeys.documents(kbId), { directory_id: directoryId }],
    queryFn: () => listDocuments(kbId, directoryId ? { directory_id: directoryId, page: 1, page_size: 100 } : { page: 1, page_size: 100 }),
  });

  if (query.isPending) return <LoadingState label="正在加载文档" />;
  if (query.error) return <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />;

  const documents = query.data?.items ?? [];
  if (documents.length === 0) {
    return <EmptyState title="暂无文档" description={directoryId ? '该目录下还没有文档。' : '点击右上角「导入文档」开始导入。'} />;
  }

  return (
    <List disablePadding dense>
      {documents.map((document) => (
        <ListItemButton key={document.id} selected={selectedId === document.id} onClick={() => onSelect(document.id)}>
          <ListItemText
            primary={document.title}
            secondary={`${processingLabel[document.processing_status]} · ${document.source_type}`}
          />
          <Box component="span" sx={{ display: 'inline-flex' }}>
            <Tooltip title="重试处理"><IconButton size="small" disabled={document.processing_status !== 'failed'} onClick={(event) => { event.stopPropagation(); onRetry(document); }}><RefreshOutlined fontSize="small" /></IconButton></Tooltip>
          </Box>
          <Tooltip title="重新索引"><IconButton size="small" onClick={(event) => { event.stopPropagation(); onReindex(document); }}><CachedOutlined fontSize="small" /></IconButton></Tooltip>
          <Tooltip title="删除"><IconButton size="small" color="error" onClick={(event) => { event.stopPropagation(); onDelete(document); }}><DeleteOutlineOutlined fontSize="small" /></IconButton></Tooltip>
        </ListItemButton>
      ))}
    </List>
  );
}

function ImportTasks({ kbId }: { kbId: string }) {
  const query = useQuery({
    queryKey: ['knowledge-bases', kbId, 'import-tasks'],
    queryFn: () => listImportTasks(kbId, { page: 1, page_size: 20 }),
    refetchInterval: (queryResult) => {
      const hasActive = queryResult.state.data?.items.some((task) => task.status === 'pending' || task.status === 'running');
      return hasActive ? 3000 : false;
    },
  });
  const queryClient = useQueryClient();
  const retry = useMutation({
    mutationFn: (taskId: string) => retryImportTask(taskId),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['knowledge-bases', kbId, 'import-tasks'] }),
  });

  if (query.isPending) return <LoadingState label="正在加载导入任务" />;
  if (query.error) return <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />;
  const tasks = query.data?.items ?? [];
  if (tasks.length === 0) return null;

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Typography variant="subtitle2" color="text.secondary" mb={1}>导入任务</Typography>
      <Stack spacing={1}>
        {tasks.map((task) => (
          <Stack key={task.id} direction="row" alignItems="center" spacing={1}>
            <Typography variant="body2" sx={{ flexGrow: 1, minWidth: 0, overflowWrap: 'anywhere' }}>
              {task.file_name || task.source_url || task.id}
            </Typography>
            <Chip size="small" color={task.status === 'failed' ? 'error' : task.status === 'succeeded' ? 'success' : 'default'} label={taskLabel[task.status]} />
            {task.failure_reason && <Typography variant="caption" color="error" noWrap>{task.failure_reason}</Typography>}
            {task.status === 'failed' && (
              <Tooltip title="重试导入"><IconButton size="small" onClick={() => retry.mutate(task.id)}><RefreshOutlined fontSize="small" /></IconButton></Tooltip>
            )}
          </Stack>
        ))}
      </Stack>
    </Paper>
  );
}

export function DocumentWorkspaceContent({
  status,
  kbId,
  documentId,
}: {
  status: CapabilityStatus;
  kbId: string;
  documentId?: string;
}) {
  const [importOpen, setImportOpen] = useState(false);
  const [directoryId, setDirectoryId] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(documentId ?? null);
  const [notice, setNotice] = useState('');
  const [actionError, setActionError] = useState<Error | null>(null);
  const queryClient = useQueryClient();
  const enabled = status === 'available';

  const treeQuery = useQuery({
    queryKey: ['knowledge-bases', kbId, 'directories'],
    queryFn: () => getDirectoryTree(kbId),
    enabled,
  });
  const documentQuery = useQuery({
    queryKey: ['documents', selectedId],
    queryFn: () => getDocument(selectedId as string),
    enabled: enabled && Boolean(selectedId),
  });
  const createDir = useMutation({
    mutationFn: (name: string) => createDirectory(kbId, { name }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['knowledge-bases', kbId, 'directories'] });
      setNotice('目录已创建');
    },
    onError: (error) => setActionError(error as Error),
  });
  const retryDoc = useMutation({
    mutationFn: (documentIdValue: string) => retryDocumentProcessing(documentIdValue),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: ['documents'] });
      setNotice('已触发重试');
    },
    onError: (error) => setActionError(error as Error),
  });
  const reindex = useMutation({
    mutationFn: (documentIdValue: string) => reindexDocument(documentIdValue),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: ['documents'] });
      setNotice('已触发重新索引');
    },
    onError: (error) => setActionError(error as Error),
  });
  const removeDoc = useMutation({
    mutationFn: (documentIdValue: string) => deleteDocument(documentIdValue),
    onSuccess: (result, documentIdValue) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: ['documents'] });
      if (selectedId === documentIdValue) setSelectedId(null);
      setNotice('文档已删除');
    },
    onError: (error) => setActionError(error as Error),
  });

  return (
    <Stack spacing={2}>
      <Stack direction="row" alignItems="center">
        <Box sx={{ flexGrow: 1 }}>
          <Typography component="h2" variant="h5" fontWeight={750}>文档工作区</Typography>
          <Typography color="text.secondary">目录、处理状态与清洗后的只读正文。</Typography>
        </Box>
        <Button
          startIcon={<UploadFileOutlined />}
          variant="contained"
          disabled={!enabled}
          onClick={() => setImportOpen(true)}
        >
          导入文档
        </Button>
      </Stack>

      {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
      {actionError && <Alert severity="error" onClose={() => setActionError(null)}>操作失败：{actionError.message}</Alert>}

      {!enabled && (
        <UnavailableState
          title="文档后端待接入"
          description="只读浏览将在文档接口可用后自动启用；当前不会加载目录或正文。"
          capability="document"
        />
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
              onCreateDirectory={(name) => createDir.mutate(name)}
            />
          }
        >
          <Stack spacing={2}>
            <Paper variant="outlined" sx={{ maxHeight: 420, overflow: 'auto' }}>
              <DocumentList
                kbId={kbId}
                directoryId={directoryId}
                selectedId={selectedId}
                onSelect={setSelectedId}
                onDelete={(document) => removeDoc.mutate(document.id)}
                onRetry={(document) => retryDoc.mutate(document.id)}
                onReindex={(document) => reindex.mutate(document.id)}
              />
            </Paper>
            <ImportTasks kbId={kbId} />
            {!selectedId && (
              <UnavailableState title="选择一篇文档" description="从左侧目录选择文档以查看只读正文。" capability="document" />
            )}
            {selectedId && documentQuery.isPending && <LoadingState label="正在加载文档" />}
            {selectedId && documentQuery.error && <ErrorState error={documentQuery.error as Error} onRetry={() => void documentQuery.refetch()} />}
            {documentQuery.data && <DocumentViewer document={documentQuery.data} />}
          </Stack>
        </DocumentWorkspace>
      )}
      <ImportDrawer
        open={importOpen}
        onClose={() => setImportOpen(false)}
        disabled={!enabled}
        kbId={kbId}
        directories={treeQuery.data ?? []}
      />
    </Stack>
  );
}

export function DocumentWorkspacePage() {
  const { kbId = '', documentId } = useParams();
  return <DocumentWorkspaceContent status={capabilities.document} kbId={kbId} documentId={documentId} />;
}
