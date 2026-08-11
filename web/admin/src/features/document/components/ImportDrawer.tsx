import {
  Alert,
  Box,
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
import JSZip from 'jszip';
import { queryKeys } from '@/api/queryKeys';
import { errorMessage } from '@/api/errors';
import { importFiles, importURL, scanImportTask, startImportTask, uploadTaskAttachments } from '../api';
import { flattenDirectories } from '../directoryOptions';
import type { DirectoryNode, ImageScanResult, ImageRefStatus, ImportSubmission } from '../types';

const MAX_FILES = 20;
const MAX_FILE_SIZE = 50 * 1024 * 1024;
const allowedExtensions = ['.md', '.txt', '.pdf', '.docx', '.xlsx', '.pptx', '.jpg', '.jpeg', '.png', '.bmp', '.tiff', '.tif', '.gif', '.webp'];
const imageExtensions = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.svg'];

// 浏览器目录选择（webkitdirectory 为非标准属性，这里显式声明）。
interface DirectoryInputElement extends HTMLInputElement {
  webkitdirectory?: string;
  directory?: string;
}

interface FolderFile extends File {
  webkitRelativePath?: string;
}

// 上传后待确认的任务（Markdown 需扫描图片引用，补传后可开始解析）。
interface PendingTask {
  taskId: string;
  fileName: string;
  scan?: ImageScanResult;
  scanError?: string;
  scanning: boolean;
  started: boolean;
}

const statusMeta: Record<ImageRefStatus, { label: string; color: 'success' | 'default' | 'warning' }> = {
  inline: { label: '内联', color: 'success' },
  network: { label: '网络', color: 'success' },
  matched: { label: '已匹配', color: 'success' },
  pending: { label: '待补传', color: 'warning' },
};

export function ImportDrawer({ open, onClose, disabled, kbId, directories }: {
  open: boolean;
  onClose: () => void;
  disabled: boolean;
  kbId: string;
  directories: DirectoryNode[];
}) {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const folderInputRef = useRef<DirectoryInputElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);
  const [sourceMode, setSourceMode] = useState<'file' | 'url'>('file');
  const [files, setFiles] = useState<File[]>([]);
  const [packing, setPacking] = useState(false);
  const [sourceURL, setSourceURL] = useState('');
  const [directoryId, setDirectoryId] = useState('');
  const [duplicatePolicy, setDuplicatePolicy] = useState<'create_new' | 'skip'>('skip');
  const [notice, setNotice] = useState('');
  const [validationError, setValidationError] = useState('');
  const [pendingTasks, setPendingTasks] = useState<PendingTask[]>([]);
  const [attachmentFor, setAttachmentFor] = useState<string | null>(null);
  const [pendingImages, setPendingImages] = useState<File[]>([]);

  const uploadMutation = useMutation({
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
      setFiles([]);
      setSourceURL('');
      if (fileInputRef.current) fileInputRef.current.value = '';
      if (folderInputRef.current) folderInputRef.current.value = '';
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
      if (sourceMode === 'url') {
        setNotice(`已创建抓取任务：${tasks.map((task) => task.file_name).join('、')}`);
        return;
      }
      // Markdown/ZIP 任务需扫描确认；其余（pdf/docx/txt）已自动开始处理。
      const pending: PendingTask[] = tasks.map((task) => ({
        taskId: task.task_id, fileName: task.file_name, scanning: false, started: false,
      }));
      setPendingTasks(pending);
      pending.forEach((task) => void refreshScan(task.taskId));
    },
  });

  const startMutation = useMutation({
    mutationFn: async (): Promise<void> => {
      await Promise.all(pendingTasks.map(async (task) => {
        if (task.started) return;
        try {
          await startImportTask(task.taskId);
          setPendingTasks((prev) => prev.map((item) =>
            item.taskId === task.taskId ? { ...item, started: true } : item));
        } catch {
          // 非 Markdown 任务已自动进入处理，start 失败可忽略。
        }
      }));
    },
    onSuccess: () => {
      setNotice('导入任务已开始处理');
      setPendingTasks([]);
      void queryClient.invalidateQueries({ queryKey: queryKeys.documents(kbId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
    },
  });

  const attachmentMutation = useMutation({
    mutationFn: async ({ taskId, images }: { taskId: string; images: File[] }): Promise<void> => {
      await uploadTaskAttachments(taskId, images);
    },
    onSuccess: (_data, { taskId }) => {
      setPendingImages([]);
      setAttachmentFor(null);
      setNotice('图片已补传，正在重新扫描…');
      void refreshScan(taskId);
    },
  });

  async function refreshScan(taskId: string) {
    setPendingTasks((prev) => prev.map((item) =>
      item.taskId === taskId ? { ...item, scanning: true, scanError: undefined } : item));
    try {
      const result = await scanImportTask(taskId);
      setPendingTasks((prev) => prev.map((item) =>
        item.taskId === taskId ? { ...item, scan: result, scanning: false } : item));
    } catch (err) {
      // 非 Markdown 文档（pdf/docx/txt）扫描会失败，忽略并视为已自动开始。
      setPendingTasks((prev) => prev.map((item) =>
        item.taskId === taskId ? { ...item, scanError: errorMessage(err), scanning: false } : item));
    }
  }

  function selectImages(selected: File[]) {
    const invalid = selected.find((file) => !imageExtensions.some((extension) => file.name.toLowerCase().endsWith(extension)));
    if (invalid) {
      setValidationError(`仅支持图片补传：${invalid.name}`);
      return;
    }
    const oversized = selected.find((file) => file.size <= 0 || file.size > 32 * 1024 * 1024);
    if (oversized) {
      setValidationError(`${oversized.name} 为空或超过 32 MB`);
      return;
    }
    setValidationError('');
    setPendingImages(selected);
  }

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

  // 选择文件夹：浏览器允许读取目录内全部文件，前端打包为 zip 上传，
  // 后端按文件名（basename）匹配 Markdown 中的本机绝对路径图片引用。
  async function selectFolder(selected: File[]) {
    const folderFiles = selected as FolderFile[];
    const main = folderFiles.find((file) =>
      allowedExtensions.some((extension) => file.name.toLowerCase().endsWith(extension)));
    if (!main) {
      setValidationError('文件夹中没有 Markdown、TXT、PDF 或 DOCX 文档');
      return;
    }
    const unsupported = folderFiles.find((file) => {
      const name = file.name.toLowerCase();
      const supported = allowedExtensions.some((extension) => name.endsWith(extension))
        || imageExtensions.some((extension) => name.endsWith(extension));
      return !supported;
    });
    if (unsupported) {
      setValidationError(`文件夹中含不支持的文件：${unsupported.name}`);
      return;
    }
    const oversized = folderFiles.find((file) => file.size <= 0 || file.size > MAX_FILE_SIZE);
    if (oversized) {
      setValidationError(`${oversized.name} 为空或超过 50 MB`);
      return;
    }
    setPacking(true);
    try {
      const zip = new JSZip();
      for (const file of folderFiles) {
        // 去掉所选根目录名，保留目录内相对路径。
        const parts = (file.webkitRelativePath || file.name).split('/');
        const relative = parts.slice(1).join('/') || file.name;
        zip.file(relative, file);
      }
      const blob = await zip.generateAsync({ type: 'blob' });
      const folderName = (folderFiles[0].webkitRelativePath || 'import').split('/')[0] || 'import';
      const zipFile = new File([blob], `${folderName}.zip`, { type: 'application/zip' });
      setFiles([zipFile]);
      setValidationError('');
      setNotice(`已打包 ${folderFiles.length} 个文件为 ${zipFile.name}`);
    } catch {
      setValidationError('打包文件夹失败，请重试');
    } finally {
      setPacking(false);
    }
  }

  const pendingCount = pendingTasks.filter((task) => !task.started).length;
  const busy = uploadMutation.isPending || startMutation.isPending || attachmentMutation.isPending || packing;
  const canUpload = sourceMode === 'file' ? files.length > 0 : /^https?:\/\//i.test(sourceURL.trim());

  return (
    <Drawer anchor="right" open={open} onClose={onClose}>
      <Stack spacing={2} sx={{ width: 480, p: 3 }}>
        <Typography variant="h6">导入知识文档</Typography>
        <ToggleButtonGroup
          exclusive
          fullWidth
          value={sourceMode}
          onChange={(_, value: 'file' | 'url' | null) => value && setSourceMode(value)}
          disabled={disabled || busy}
        >
          <ToggleButton value="file">文件上传</ToggleButton>
          <ToggleButton value="url">URL 抓取</ToggleButton>
        </ToggleButtonGroup>
        {notice && <Alert severity="success" onClose={() => setNotice('')}>{notice}</Alert>}
        {validationError && <Alert severity="warning" onClose={() => setValidationError('')}>{validationError}</Alert>}
        {uploadMutation.error && <Alert severity="error">{errorMessage(uploadMutation.error)}</Alert>}
        {startMutation.error && <Alert severity="error">开始导入失败：{errorMessage(startMutation.error)}</Alert>}
        {attachmentMutation.error && <Alert severity="error">图片补传失败：{errorMessage(attachmentMutation.error)}</Alert>}
        {busy && <LinearProgress />}

        {sourceMode === 'file' ? (
          <>
            <Typography color="text.secondary">支持 Markdown、TXT、PDF、DOCX、XLSX、PPTX、图片（JPG/PNG/BMP/TIFF）、ZIP；Markdown 上传后自动扫描图片引用，可随时补传本机图片。</Typography>
            <Stack direction="row" spacing={1}>
              <Button variant="outlined" disabled={disabled || packing || pendingCount > 0} onClick={() => fileInputRef.current?.click()}>选择文件</Button>
              <Button variant="outlined" disabled={disabled || packing || pendingCount > 0} onClick={() => folderInputRef.current?.click()}>
                {packing ? '正在打包…' : '选择文件夹'}
              </Button>
            </Stack>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              hidden
              accept=".md,.txt,.pdf,.docx,.xlsx,.pptx,.zip,.jpg,.jpeg,.png,.bmp,.tiff,.tif,.gif,.webp,text/markdown,text/plain,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.openxmlformats-officedocument.presentationml.presentation,application/zip,image/jpeg,image/png,image/bmp,image/tiff,image/gif,image/webp"
              onChange={(event) => selectFiles(Array.from(event.target.files ?? []))}
            />
            <input
              ref={folderInputRef}
              type="file"
              multiple
              hidden
              {...({ webkitdirectory: '', directory: '' } as Record<string, string>)}
              onChange={(event) => void selectFolder(Array.from(event.target.files ?? []))}
            />
            {files.length > 0 && (
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                {files.map((file) => (
                  <Chip key={`${file.name}-${file.size}`} size="small" label={`${file.name} (${(file.size / 1024).toFixed(1)} KB)`} onDelete={() => setFiles((prev) => prev.filter((item) => item !== file))} />
                ))}
              </Stack>
            )}
            {pendingTasks.length > 0 && (
              <Stack spacing={1.5}>
                {pendingTasks.map((task) => (
                  <Box key={task.taskId} sx={{ border: 1, borderColor: 'divider', borderRadius: 1, p: 1.5 }}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Typography variant="body2" fontWeight={700} sx={{ flexGrow: 1 }}>{task.fileName}</Typography>
                      {task.started
                        ? <Chip size="small" color="primary" label="已开始" />
                        : task.scanError
                          ? <Chip size="small" color="default" label="已自动处理" />
                          : <Chip size="small" color="info" label="待确认" />}
                    </Stack>
                    {task.scan && task.scan.refs.length > 0 && (
                      <Stack spacing={0.5} mt={1}>
                        {task.scan.refs.map((ref, index) => (
                          <Stack key={`${ref.ref}-${index}`} direction="row" spacing={1} alignItems="center">
                            <Chip
                              size="small"
                              variant="outlined"
                              color={statusMeta[ref.status].color}
                              label={statusMeta[ref.status].label}
                            />
                            <Typography variant="caption" sx={{ wordBreak: 'break-all' }}>{ref.ref}</Typography>
                          </Stack>
                        ))}
                      </Stack>
                    )}
                    {task.scan?.refs.length === 0 && !task.started && (
                      <Typography variant="caption" color="text.secondary" mt={1}>未发现图片引用</Typography>
                    )}
                    {task.scan?.refs.some((ref) => ref.status === 'pending') && !task.started && (
                      <Stack direction="row" spacing={1} mt={1} alignItems="center">
                        <Button
                          size="small"
                          variant="outlined"
                          disabled={busy}
                          onClick={() => {
                            setAttachmentFor(task.taskId);
                            setPendingImages([]);
                            imageInputRef.current?.click();
                          }}
                        >
                          补传图片
                        </Button>
                        <input
                          ref={imageInputRef}
                          type="file"
                          multiple
                          hidden
                          accept={imageExtensions.join(',')}
                          onChange={(event) => selectImages(Array.from(event.target.files ?? []))}
                        />
                        {attachmentFor === task.taskId && pendingImages.length > 0 && (
                          <Stack direction="row" spacing={1} alignItems="center">
                            <Typography variant="caption">{pendingImages.length} 张待补传</Typography>
                            <Button
                              size="small"
                              variant="contained"
                              disabled={attachmentMutation.isPending}
                              onClick={() => attachmentMutation.mutate({ taskId: task.taskId, images: pendingImages })}
                            >
                              {attachmentMutation.isPending ? '上传中…' : '确认补传'}
                            </Button>
                          </Stack>
                        )}
                      </Stack>
                    )}
                    {task.scan && task.scan.refs.some((ref) => ref.status === 'pending') && (
                      <Typography variant="caption" color="warning.main" mt={0.5} display="block">
                        未补传的图片将在解析时标记 unresolved，不影响正文导入
                      </Typography>
                    )}
                  </Box>
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
              disabled={disabled || busy}
            />
          </>
        )}

        <TextField select label="目标目录" value={directoryId} onChange={(event) => setDirectoryId(event.target.value)} disabled={disabled || busy}>
          <MenuItem value="">（不归入目录）</MenuItem>
          {options.map((option) => <MenuItem key={option.id} value={option.id}>{'　'.repeat(option.depth)}{option.name}</MenuItem>)}
        </TextField>
        <TextField select label="重复策略" value={duplicatePolicy} onChange={(event) => setDuplicatePolicy(event.target.value as 'create_new' | 'skip')} disabled={disabled || busy}>
          <MenuItem value="skip">重复内容跳过（推荐）</MenuItem>
          <MenuItem value="create_new">重复内容创建新文档</MenuItem>
        </TextField>
        {pendingTasks.length > 0 ? (
          <Button
            variant="contained"
            disabled={busy || pendingCount === 0}
            onClick={() => startMutation.mutate()}
          >
            {startMutation.isPending ? '正在开始…' : `开始导入（${pendingCount} 个待确认任务）`}
          </Button>
        ) : (
          <Button variant="contained" disabled={disabled || !canUpload || busy} onClick={() => uploadMutation.mutate()}>
            {uploadMutation.isPending ? '正在上传…' : sourceMode === 'url' ? '开始抓取' : '上传并扫描'}
          </Button>
        )}
      </Stack>
    </Drawer>
  );
}
