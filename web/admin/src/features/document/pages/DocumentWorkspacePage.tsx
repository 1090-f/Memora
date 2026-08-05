import UploadFileOutlined from '@mui/icons-material/UploadFileOutlined';
import { Box, Button, Stack, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { DocumentWorkspace } from '@/layouts/DocumentWorkspace';
import { getDirectoryTree, getDocument } from '../api';
import { DocumentViewer } from '../components/DocumentViewer';
import { ImportDrawer } from '../components/ImportDrawer';
import { KnowledgeTree } from '../components/KnowledgeTree';

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
  const enabled = status === 'available';
  const treeQuery = useQuery({
    queryKey: ['knowledge-bases', kbId, 'directories'],
    queryFn: () => getDirectoryTree(kbId),
    enabled,
  });
  const documentQuery = useQuery({
    queryKey: ['documents', documentId],
    queryFn: () => getDocument(documentId as string),
    enabled: enabled && Boolean(documentId),
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
        <DocumentWorkspace sidebar={<KnowledgeTree nodes={treeQuery.data} />}>
          {!documentId && (
            <UnavailableState title="选择一篇文档" description="从左侧目录选择文档以查看只读正文。" capability="document" />
          )}
          {documentId && documentQuery.isPending && <LoadingState label="正在加载文档" />}
          {documentId && documentQuery.error && <ErrorState error={documentQuery.error as Error} onRetry={() => void documentQuery.refetch()} />}
          {documentQuery.data && <DocumentViewer document={documentQuery.data} />}
        </DocumentWorkspace>
      )}
      <ImportDrawer open={importOpen} onClose={() => setImportOpen(false)} disabled={!enabled} />
    </Stack>
  );
}

export function DocumentWorkspacePage() {
  const { kbId = '', documentId } = useParams();
  return <DocumentWorkspaceContent status={capabilities.document} kbId={kbId} documentId={documentId} />;
}
