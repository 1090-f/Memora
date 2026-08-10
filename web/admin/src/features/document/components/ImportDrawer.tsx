import { Alert, Button, Chip, Drawer, LinearProgress, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { queryKeys } from '@/api/queryKeys';
import { errorMessage } from '@/api/errors';
import { importFiles } from '../api';
import { flattenDirectories } from '../directoryOptions';
import type { DirectoryNode } from '../types';

const MAX_FILES = 20;
const MAX_FILE_SIZE = 50 * 1024 * 1024;
// 任务包 00～06 只完成 Markdown/TXT 的稳定加工链路，其他格式留待任务包 09。
const allowedExtensions = ['.md', '.txt'];

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
  const [duplicatePolicy, setDuplicatePolicy] = useState<'create_new' | 'skip'>('skip');
  const [notice, setNotice] = useState('');
  const [validationError, setValidationError] = useState('');
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
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
    },
  });

  const options = flattenDirectories(directories);

  function selectFiles(selected: File[]) {
    // 前端先做与后端一致的数量、类型和大小校验，减少无效上传流量。
    if (selected.length > MAX_FILES) {
      setFiles([]);
      setValidationError(`单次最多选择 ${MAX_FILES} 个文件`);
      return;
    }
    const invalidType = selected.find((file) => !allowedExtensions.some((extension) => file.name.toLowerCase().endsWith(extension)));
    if (invalidType) {
      setFiles([]);
      setValidationError(`暂不支持 ${invalidType.name}，任务包 00–06 仅开放 Markdown 和 TXT`);
      return;
    }
    const invalidSize = selected.find((file) => file.size <= 0 || file.size > MAX_FILE_SIZE);
    if (invalidSize) {
      setFiles([]);
      setValidationError(`${invalidSize.name} 为空或超过 50 MB`);
      return;
    }
    setValidationError('');
    setNotice('');
    setFiles(selected);
  }

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Stack spacing={2} sx={{ width: 420, p: 3 }}>
        <Typography variant="h6">导入文档</Typography>
        <Typography color="text.secondary">支持 Markdown 和 TXT，单文件最大 50 MB、单次最多 20 个；上传后由 Worker 异步解析、分段并建立索引。</Typography>
        {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
        {validationError && <Alert severity="warning" onClose={() => setValidationError('')}>{validationError}</Alert>}
        {mutation.error && <Alert severity="error">{errorMessage(mutation.error)}</Alert>}
        {mutation.isPending && <LinearProgress />}
        <Button variant="contained" disabled={disabled} onClick={() => fileInputRef.current?.click()}>
          选择文件
        </Button>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          hidden
          accept=".md,.txt,text/markdown,text/plain"
          onChange={(event) => selectFiles(Array.from(event.target.files ?? []))}
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
          <MenuItem value="skip">重复内容跳过（推荐）</MenuItem>
          <MenuItem value="create_new">重复内容创建新文档</MenuItem>
        </TextField>
        <Typography color="text.secondary" variant="body2">PDF、DOCX 与 URL 导入将在任务包 09 完成后开放。</Typography>
        <Button
          variant="contained"
          disabled={disabled || files.length === 0 || mutation.isPending}
          onClick={() => mutation.mutate()}
        >
          {mutation.isPending ? '正在上传…' : '开始导入'}
        </Button>
      </Stack>
    </Drawer>
  );
}
