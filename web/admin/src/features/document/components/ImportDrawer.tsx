import {
  Alert,
  Button,
  Chip,
  Drawer,
  LinearProgress,
  MenuItem,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { queryKeys } from '@/api/queryKeys';
import { errorMessage } from '@/api/errors';
import { importFiles, importURL } from '../api';
import { flattenDirectories } from '../directoryOptions';
import type { DirectoryNode, ImportSubmission } from '../types';

const MAX_FILES = 20;
const MAX_FILE_SIZE = 50 * 1024 * 1024;
const allowedExtensions = ['.md', '.txt', '.pdf', '.docx'];

export function ImportDrawer({ open, onClose, disabled, kbId, directories }: {
  open: boolean;
  onClose: () => void;
  disabled: boolean;
  kbId: string;
  directories: DirectoryNode[];
}) {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [sourceMode, setSourceMode] = useState<'file' | 'url'>('file');
  const [files, setFiles] = useState<File[]>([]);
  const [sourceURL, setSourceURL] = useState('');
  const [directoryId, setDirectoryId] = useState('');
  const [duplicatePolicy, setDuplicatePolicy] = useState<'create_new' | 'skip'>('skip');
  const [notice, setNotice] = useState('');
  const [validationError, setValidationError] = useState('');
  const mutation = useMutation({
    mutationFn: async (): Promise<ImportSubmission[]> => {
      if (sourceMode === 'url') {
        const task = await importURL(kbId, {
          url: sourceURL.trim(),
          directory_id: directoryId || undefined,
          duplicate_policy: duplicatePolicy,
        });
        return [task];
      }
      const formData = new FormData();
      files.forEach((file) => formData.append('files', file));
      if (directoryId) formData.append('directory_id', directoryId);
      formData.append('duplicate_policy', duplicatePolicy);
      const result = await importFiles(kbId, formData);
      return result.tasks;
    },
    onSuccess: (tasks) => {
      const names = tasks.map((task) => task.file_name).join('、');
      setNotice(`已创建 ${tasks.length} 个导入任务：${names}`);
      setFiles([]);
      setSourceURL('');
      if (fileInputRef.current) fileInputRef.current.value = '';
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
    },
  });

  const options = flattenDirectories(directories);

  function selectFiles(selected: File[]) {
    if (selected.length > MAX_FILES) {
      setFiles([]);
      setValidationError(`单次最多选择 ${MAX_FILES} 个文件`);
      return;
    }
    const invalidType = selected.find((file) => !allowedExtensions.some((extension) => file.name.toLowerCase().endsWith(extension)));
    if (invalidType) {
      setFiles([]);
      setValidationError(`不支持 ${invalidType.name}，请选择 Markdown、TXT、PDF 或 DOCX`);
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

  const canSubmit = sourceMode === 'file' ? files.length > 0 : /^https?:\/\//i.test(sourceURL.trim());

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Stack spacing={2} sx={{ width: 440, p: 3 }}>
        <Typography variant="h6">导入知识文档</Typography>
        <ToggleButtonGroup
          exclusive
          fullWidth
          value={sourceMode}
          onChange={(_, value: 'file' | 'url' | null) => value && setSourceMode(value)}
          disabled={disabled || mutation.isPending}
        >
          <ToggleButton value="file">文件上传</ToggleButton>
          <ToggleButton value="url">URL 抓取</ToggleButton>
        </ToggleButtonGroup>
        {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
        {validationError && <Alert severity="warning" onClose={() => setValidationError('')}>{validationError}</Alert>}
        {mutation.error && <Alert severity="error">{errorMessage(mutation.error)}</Alert>}
        {mutation.isPending && <LinearProgress />}

        {sourceMode === 'file' ? (
          <>
            <Typography color="text.secondary">支持 Markdown、TXT、PDF、DOCX；单文件最大 50 MB，单次最多 20 个。</Typography>
            <Button variant="outlined" disabled={disabled} onClick={() => fileInputRef.current?.click()}>选择文件</Button>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              hidden
              accept=".md,.txt,.pdf,.docx,text/markdown,text/plain,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document"
              onChange={(event) => selectFiles(Array.from(event.target.files ?? []))}
            />
            {files.length > 0 && (
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                {files.map((file) => (
                  <Chip key={`${file.name}-${file.size}`} size="small" label={`${file.name} (${(file.size / 1024).toFixed(1)} KB)`} onDelete={() => setFiles((prev) => prev.filter((item) => item !== file))} />
                ))}
              </Stack>
            )}
          </>
        ) : (
          <>
            <Typography color="text.secondary">网页由 Worker 异步抓取；服务端会执行 SSRF、重定向、内容类型、大小和超时校验。</Typography>
            <TextField
              label="网页 URL"
              value={sourceURL}
              onChange={(event) => setSourceURL(event.target.value)}
              placeholder="https://example.com/article"
              error={sourceURL !== '' && !/^https?:\/\//i.test(sourceURL.trim())}
              helperText="仅支持 HTTP/HTTPS 公网地址"
              disabled={disabled || mutation.isPending}
            />
          </>
        )}

        <TextField select label="目标目录" value={directoryId} onChange={(event) => setDirectoryId(event.target.value)} disabled={disabled || mutation.isPending}>
          <MenuItem value="">（不归入目录）</MenuItem>
          {options.map((option) => <MenuItem key={option.id} value={option.id}>{'　'.repeat(option.depth)}{option.name}</MenuItem>)}
        </TextField>
        <TextField select label="重复策略" value={duplicatePolicy} onChange={(event) => setDuplicatePolicy(event.target.value as 'create_new' | 'skip')} disabled={disabled || mutation.isPending}>
          <MenuItem value="skip">重复内容跳过（推荐）</MenuItem>
          <MenuItem value="create_new">重复内容创建新文档</MenuItem>
        </TextField>
        <Button variant="contained" disabled={disabled || !canSubmit || mutation.isPending} onClick={() => mutation.mutate()}>
          {mutation.isPending ? '正在创建任务…' : sourceMode === 'url' ? '开始抓取' : '开始导入'}
        </Button>
      </Stack>
    </Drawer>
  );
}
