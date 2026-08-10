import { Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, MenuItem, Stack, TextField } from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { errorMessage } from '@/api/errors';
import { queryKeys } from '@/api/queryKeys';
import { createManualDocument } from '../api';
import { flattenDirectories } from '../directoryOptions';
import type { DirectoryNode } from '../types';

export function CreateManualDocumentDialog({ open, onClose, kbId, directories, initialDirectoryId, onCreated }: {
  open: boolean;
  onClose: () => void;
  kbId: string;
  directories: DirectoryNode[];
  initialDirectoryId: string | null;
  onCreated: (documentId: string) => void;
}) {
  const queryClient = useQueryClient();
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [sourceUrl, setSourceUrl] = useState('');
  const [directoryId, setDirectoryId] = useState('');
  const options = flattenDirectories(directories);

  useEffect(() => {
    if (open) setDirectoryId(initialDirectoryId ?? '');
  }, [initialDirectoryId, open]);

  const mutation = useMutation({
    // 手工文档直接调用文档 CRUD 接口，不伪造文件导入任务。
    mutationFn: () => createManualDocument(kbId, {
      title: title.trim(),
      content: content || undefined,
      directory_id: directoryId || undefined,
      source_type: 'manual',
      source_url: sourceUrl.trim() || undefined,
    }),
    onSuccess: (document) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      setTitle('');
      setContent('');
      setSourceUrl('');
      onCreated(document.id);
      onClose();
    },
  });

  return (
    <Dialog open={open} onClose={mutation.isPending ? undefined : onClose} fullWidth maxWidth="md">
      <DialogTitle>新建手工文档</DialogTitle>
      <DialogContent>
        <Stack spacing={2} mt={1}>
          <TextField label="标题" required value={title} onChange={(event) => setTitle(event.target.value)} inputProps={{ maxLength: 500 }} />
          <TextField
            label="正文"
            multiline
            minRows={10}
            value={content}
            onChange={(event) => setContent(event.target.value)}
            helperText={`${new Blob([content]).size.toLocaleString()} / 2,097,152 字节`}
            error={new Blob([content]).size > 2 * 1024 * 1024}
          />
          <TextField select label="目标目录" value={directoryId} onChange={(event) => setDirectoryId(event.target.value)}>
            <MenuItem value="">（不归入目录）</MenuItem>
            {options.map((option) => (
              <MenuItem key={option.id} value={option.id}>{'　'.repeat(option.depth)}{option.name}</MenuItem>
            ))}
          </TextField>
          <TextField label="来源 URL（可选）" type="url" value={sourceUrl} onChange={(event) => setSourceUrl(event.target.value)} inputProps={{ maxLength: 2000 }} />
          {mutation.error && <Alert severity="error">{errorMessage(mutation.error)}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={mutation.isPending}>取消</Button>
        <Button
          variant="contained"
          disabled={title.trim() === '' || new Blob([content]).size > 2 * 1024 * 1024 || mutation.isPending}
          onClick={() => mutation.mutate()}
        >
          {mutation.isPending ? '创建中…' : '创建'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
