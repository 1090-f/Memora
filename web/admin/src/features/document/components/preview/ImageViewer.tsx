import { Alert, Box, Typography } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { errorMessage } from '@/api/errors';
import { getDocumentPreviewBlob } from '../../api';

export function ImageViewer({ documentId, contentUrl, title }: { documentId: string; contentUrl: string; title: string }) {
  const query = useQuery({
    queryKey: ['documents', documentId, 'preview-image', contentUrl],
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

  if (query.isPending) return <Typography color="text.secondary">正在加载原图…</Typography>;
  if (query.error) return <Alert severity="warning">预览资源读取失败：{errorMessage(query.error)}</Alert>;
  if (!objectUrl) return null;
  return <Box textAlign="center"><img src={objectUrl} alt={title} style={{ maxWidth: '100%', maxHeight: '72vh', objectFit: 'contain' }} /></Box>;
}