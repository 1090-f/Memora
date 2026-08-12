import { Alert, Box, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { errorMessage } from '@/api/errors';
import { getDocumentPreviewBlob } from '../../api';

export function BlobViewer({ documentId, type, contentUrl, title }: { documentId: string; type: 'pdf' | 'image'; contentUrl: string; title: string }) {
  const query = useQuery({
    queryKey: ['documents', documentId, 'preview-blob', type, contentUrl],
    queryFn: () => getDocumentPreviewBlob(contentUrl),
    staleTime: Infinity,
    retry: false,
  });
  const [objectUrl, setObjectUrl] = useState('');

  useEffect(() => {
    if (!query.data) { setObjectUrl(''); return; }
    const next = URL.createObjectURL(query.data);
    setObjectUrl(next);
    return () => URL.revokeObjectURL(next);
  }, [query.data]);

  if (query.isPending) return <Typography color="text.secondary">正在加载{type === 'pdf' ? ' PDF' : '原图'}…</Typography>;
  if (query.error) return <Alert severity="warning">预览资源读取失败：{errorMessage(query.error)}</Alert>;
  if (!objectUrl) return null;
  if (type === 'image') {
    return <Box textAlign="center"><img src={objectUrl} alt={title} style={{ maxWidth: '100%', maxHeight: '72vh', objectFit: 'contain' }} /></Box>;
  }
  return <Box sx={{ '& iframe': { width: '100%', height: '72vh', border: 'none', borderRadius: 1 } }}><iframe src={objectUrl} title={`${title} PDF 预览`} /></Box>;
}
