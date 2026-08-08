import { Alert, Button, Chip, Drawer, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { queryKeys } from '@/api/queryKeys';
import { importFiles } from '../api';
import type { DirectoryNode } from '../types';

function flattenDirectories(nodes: DirectoryNode[], depth = 0): Array<{ id: string | null; name: string; depth: number }> {
  return nodes.flatMap((node) => [
    { id: node.id, name: node.name, depth },
    ...flattenDirectories(node.children, depth + 1),
  ]);
}

export function ImportDrawer({ open, onClose, disabled, kbId, directories }: {
  open: boolean;
  onClose: () => void;
  disabled: boolean;
  kbId: string;
  directories: DirectoryNode[];
}) {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [files, setFiles] = useState<File[]>([]);
  const [directoryId, setDirectoryId] = useState('');
  const [duplicatePolicy, setDuplicatePolicy] = useState<'create_new' | 'skip'>('create_new');
  const [notice, setNotice] = useState('');
  const mutation = useMutation({
    mutationFn: () => {
      const formData = new FormData();
      files.forEach((file) => formData.append('files', file));
      if (directoryId) formData.append('directory_id', directoryId);
      formData.append('duplicate_policy', duplicatePolicy);
      return importFiles(kbId, formData);
    },
    onSuccess: (result) => {
      const names = result.tasks.map((task) => task.file_name).join('、');
      setNotice(`已创建 ${result.tasks.length} 个导入任务：${names}`);
      setFiles([]);
      if (fileInputRef.current) fileInputRef.current.value = '';
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: ['knowledge-bases', kbId, 'import-tasks'] });
    },
  });

  const options = flattenDirectories(directories);

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Stack spacing={2} sx={{ width: 420, p: 3 }}>
        <Typography variant="h6">导入文档</Typography>
        <Typography color="text.secondary">支持 md、txt、pdf、docx 文件；上传后由 Worker 异步解析、分段并建立索引。</Typography>
        {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
        {mutation.error && <Alert severity="error">{(mutation.error as Error).message}</Alert>}
        <Button variant="contained" disabled={disabled} onClick={() => fileInputRef.current?.click()}>
          选择文件
        </Button>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          hidden
          accept=".md,.txt,.pdf,.docx,.doc,.markdown"
          onChange={(event) => setFiles(Array.from(event.target.files ?? []))}
        />
        {files.length > 0 && (
          <Stack direction="row" spacing={1} flexWrap="wrap">
            {files.map((file) => (
              <Chip key={`${file.name}-${file.size}`} size="small" label={`${file.name} (${(file.size / 1024).toFixed(1)} KB)`} onDelete={() => setFiles((prev) => prev.filter((item) => item !== file))} />
            ))}
          </Stack>
        )}
        <TextField
          select
          label="目标目录"
          value={directoryId}
          onChange={(event) => setDirectoryId(event.target.value)}
          disabled={disabled}
        >
          <MenuItem value="">（不归入目录）</MenuItem>
          {options.map((option) => (
            <MenuItem key={option.id} value={option.id as string}>
              {'　'.repeat(option.depth)}{option.name}
            </MenuItem>
          ))}
        </TextField>
        <TextField
          select
          label="重复策略"
          value={duplicatePolicy}
          onChange={(event) => setDuplicatePolicy(event.target.value as 'create_new' | 'skip')}
          disabled={disabled}
        >
          <MenuItem value="create_new">同名文件创建新文档</MenuItem>
          <MenuItem value="skip">同名文件跳过</MenuItem>
        </TextField>
        <Typography color="text.secondary" variant="body2">
          URL 导入将在后端提供对应接口后启用。
        </Typography>
        <Button
          variant="contained"
          disabled={disabled || files.length === 0 || mutation.isPending}
          onClick={() => mutation.mutate()}
        >
          开始导入
        </Button>
      </Stack>
    </Drawer>
  );
}
