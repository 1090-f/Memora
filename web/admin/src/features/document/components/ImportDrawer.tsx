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
import { useCallback, useEffect, useRef, useState } from 'react';
import JSZip from 'jszip';
import { useDropzone, type FileRejection } from 'react-dropzone';
import CheckCircleOutlined from '@mui/icons-material/CheckCircleOutlined';
import CloseOutlined from '@mui/icons-material/CloseOutlined';
import CloudUploadOutlined from '@mui/icons-material/CloudUploadOutlined';
import DeleteOutlineOutlined from '@mui/icons-material/DeleteOutlineOutlined';
import ErrorOutlineOutlined from '@mui/icons-material/ErrorOutlineOutlined';
import InsertDriveFileOutlined from '@mui/icons-material/InsertDriveFileOutlined';
import ReplayOutlined from '@mui/icons-material/ReplayOutlined';
import clsx from 'clsx';
import { queryKeys } from '@/api/queryKeys';
import { errorMessage } from '@/api/errors';
import { importFiles, importURL, scanImportTask, startImportTask, uploadTaskAttachments } from '../api';
import { flattenDirectories } from '../directoryOptions';
import type { DirectoryNode, ImageScanResult, ImageRefStatus, ImportUploadResponse } from '../types';

const MAX_FILES = 20;
const MAX_FILE_SIZE = 50 * 1024 * 1024;
const imageExtensions = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.bmp', '.svg', '.tiff', '.tif'];
const primaryDocumentExtensions = ['.md', '.txt', '.pdf', '.docx', '.xlsx', '.pptx'];
const standaloneImageExtensions = imageExtensions.filter((extension) => extension !== '.svg');
const importableDocumentExtensions = [...primaryDocumentExtensions, ...standaloneImageExtensions];
const allowedExtensions = [...importableDocumentExtensions, '.zip'];

const uploadAccept: Record<string, string[]> = {
  'text/markdown': ['.md'],
  'text/plain': ['.txt'],
  'application/pdf': ['.pdf'],
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document': ['.docx'],
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet': ['.xlsx'],
  'application/vnd.openxmlformats-officedocument.presentationml.presentation': ['.pptx'],
  'application/zip': ['.zip'],
  'image/jpeg': ['.jpg', '.jpeg'],
  'image/png': ['.png'],
  'image/bmp': ['.bmp'],
  'image/tiff': ['.tiff', '.tif'],
  'image/gif': ['.gif'],
  'image/webp': ['.webp'],
};

// 浏览器目录选择（webkitdirectory 为非标准属性，在浏览器类型上扩展，与 HTMLInputElement 兼容）。
interface DirectoryInputElement extends HTMLInputElement {
  webkitdirectory: boolean;
  directory: boolean;
}

// 上传后待确认的任务（Markdown 需扫描图片引用，补传后可开始解析）。
interface PendingTask {
  taskId: string;
  fileName: string;
  sourcePath?: string;
  scan?: ImageScanResult;
  scanError?: string;
  scanning: boolean;
  started: boolean;
}

// 待上传文件队列项：queued 等待上传；uploading 上传中；success/error 为结果状态。
interface PendingFile {
  key: string;
  file: File;
  status: 'queued' | 'uploading' | 'success' | 'error';
  errorMessage?: string;
  retryable?: boolean;
  previewUrl?: string;
}

const statusMeta: Record<ImageRefStatus, { label: string; color: 'success' | 'default' | 'warning' }> = {
  inline: { label: '内联', color: 'success' },
  network: { label: '网络', color: 'success' },
  matched: { label: '已匹配', color: 'success' },
  pending: { label: '待补传', color: 'warning' },
};

const iconButtonClass =
  'rounded-lg p-1.5 text-zinc-400 transition-colors duration-200 motion-reduce:transition-none ' +
  'hover:bg-zinc-100 hover:text-zinc-900 active:scale-95 ' +
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30';

const retryButtonClass =
  'flex items-center gap-1 rounded-lg text-sm font-medium transition-colors duration-200 motion-reduce:transition-none ' +
  'bg-zinc-100 text-zinc-700 hover:bg-zinc-200 active:scale-95 px-2 py-1.5 ' +
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30';

const createPendingFile = (file: File, errorMessage?: string, retryable = false): PendingFile => ({
  key: `${file.name}-${file.size}-${file.lastModified}-${Math.random().toString(36).slice(2)}`,
  file,
  status: errorMessage ? 'error' : 'queued',
  errorMessage,
  retryable,
  previewUrl: file.type.startsWith('image/') ? URL.createObjectURL(file) : undefined,
});

export function ImportDrawer({ open, onClose, disabled, kbId, directories }: {
  open: boolean;
  onClose: () => void;
  disabled: boolean;
  kbId: string;
  directories: DirectoryNode[];
}) {
  const queryClient = useQueryClient();
  const folderInputRef = useRef<DirectoryInputElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);
  const uploadControllerRef = useRef<AbortController | null>(null);
  const filesStateRef = useRef<{ files: PendingFile[] }>({ files: [] });
  const [sourceMode, setSourceMode] = useState<'file' | 'url'>('file');
  const [pendingFiles, setPendingFiles] = useState<PendingFile[]>([]);
  const [fileImportMode, setFileImportMode] = useState<'files' | 'folder_archive'>('files');
  const [packing, setPacking] = useState(false);
  const [sourceURL, setSourceURL] = useState('');
  const [directoryId, setDirectoryId] = useState('');
  const [duplicatePolicy, setDuplicatePolicy] = useState<'create_new' | 'skip'>('skip');
  const [notice, setNotice] = useState('');
  const [validationError, setValidationError] = useState('');
  const [uploadProgress, setUploadProgress] = useState(0);
  const [cancelled, setCancelled] = useState(false);
  const [pendingTasks, setPendingTasks] = useState<PendingTask[]>([]);
  const [attachmentFor, setAttachmentFor] = useState<string | null>(null);
  const [pendingImages, setPendingImages] = useState<File[]>([]);
  filesStateRef.current.files = pendingFiles;

  useEffect(() => {
    const state = filesStateRef.current;
    return () => {
      state.files.forEach((item) => {
        if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
      });
    };
  }, []);

  const queuedFiles = pendingFiles.filter((item) => item.status === 'queued');

  const uploadMutation = useMutation({
    mutationFn: async (): Promise<ImportUploadResponse> => {
      if (sourceMode === 'url') {
        const task = await importURL(kbId, {
          url: sourceURL.trim(),
          directory_id: directoryId || undefined,
          duplicate_policy: duplicatePolicy,
        });
        return {
          batch_id: task.batch_id ?? '',
          summary: { total: 1, accepted: 1, rejected: 0 },
          tasks: [task],
          rejected: [],
        };
      }
      const formData = new FormData();
      queuedFiles.forEach((item) => formData.append('files', item.file));
      if (directoryId) formData.append('directory_id', directoryId);
      formData.append('duplicate_policy', duplicatePolicy);
      if (fileImportMode === 'folder_archive') formData.append('import_mode', 'folder_archive');
      const controller = new AbortController();
      uploadControllerRef.current = controller;
      setCancelled(false);
      setUploadProgress(0);
      setPendingFiles((prev) => prev.map((item) =>
        item.status === 'queued' ? { ...item, status: 'uploading' } : item));
      return importFiles(kbId, formData, {
        onUploadProgress: setUploadProgress,
        signal: controller.signal,
      });
    },
    onSuccess: (result) => {
      uploadControllerRef.current = null;
      setCancelled(false);
      setUploadProgress(100);
      setFileImportMode('files');
      setSourceURL('');
      if (folderInputRef.current) folderInputRef.current.value = '';
      void queryClient.invalidateQueries({ queryKey: queryKeys.importTasks(kbId) });
      if (sourceMode === 'url') {
        setPendingFiles([]);
        setNotice(`已创建抓取任务：${result.tasks.map((task) => task.file_name).join('、')}`);
        return;
      }
      const rejectedByPath = new Map(result.rejected.map((item) => [item.source_path, item.message]));
      setPendingFiles((prev) => prev.map((item) => {
        if (item.status !== 'uploading') return item;
        const message = rejectedByPath.get(item.file.name);
        if (message) return { ...item, status: 'error', errorMessage: message, retryable: true };
        return { ...item, status: 'success' };
      }));
      // 只有后端明确标记 requires_confirmation 的 Markdown/传统 ZIP 任务才进入补图确认。
      const pending: PendingTask[] = result.tasks.filter((task) => task.requires_confirmation).map((task) => ({
        taskId: task.task_id,
        fileName: task.file_name,
        sourcePath: task.source_path,
        scanning: false,
        started: false,
      }));
      setPendingTasks(pending);
      pending.forEach((task) => void refreshScan(task.taskId));
      const rejectedNotice = result.rejected.length > 0
        ? `；${result.rejected.length} 个文件未导入：${result.rejected.slice(0, 3).map((item) => item.source_path).join('、')}`
        : '';
      setNotice(`已创建 ${result.summary.accepted} 个文档任务${rejectedNotice}`);
      if (result.rejected.length > 0) {
        setValidationError(result.rejected.map((item) => `${item.source_path}：${item.message}`).join('；'));
      }
    },
    onError: () => {
      uploadControllerRef.current = null;
      setUploadProgress(0);
      if (cancelled) return;
      setPendingFiles((prev) => prev.map((item) =>
        item.status === 'uploading'
          ? { ...item, status: 'error', errorMessage: '上传失败，请重试', retryable: true }
          : item));
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

  const addSelectedFiles = useCallback((selected: File[], rejected: Array<{ file: File; message: string }> = []) => {
    setPendingFiles((prev) => {
      const hasActive = prev.some((item) => item.status === 'queued' || item.status === 'uploading');
      if (!hasActive) {
        prev.forEach((item) => {
          if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
        });
      }
      const base = hasActive ? prev : [];
      const room = Math.max(0, MAX_FILES - base.length);
      const accepted = selected.slice(0, room).map((file) => {
        const invalidType = !allowedExtensions.some((extension) => file.name.toLowerCase().endsWith(extension));
        if (invalidType) return createPendingFile(file, `不支持 ${file.name}，请选择 Markdown、TXT、PDF 或 DOCX`);
        const invalidSize = file.size <= 0 || file.size > MAX_FILE_SIZE;
        if (invalidSize) return createPendingFile(file, `${file.name} 为空或超过 50 MB`);
        return createPendingFile(file);
      });
      const overLimit = selected.slice(room).map((file) => createPendingFile(file, `超出单次 ${MAX_FILES} 个文件上限`));
      const invalid = rejected.map(({ file, message }) => createPendingFile(file, message));
      return [...base, ...accepted, ...invalid, ...overLimit];
    });
    setFileImportMode('files');
    setValidationError('');
    setNotice('');
  }, []);

  const onDropFiles = useCallback((acceptedFiles: File[], fileRejections: FileRejection[]) => {
    const rejected = fileRejections.map((rejection) => {
      const oversized = rejection.errors.some((error) => error.code === 'file-too-large' || error.code === 'file-too-small');
      return {
        file: rejection.file,
        message: oversized
          ? `${rejection.file.name} 为空或超过 50 MB`
          : `不支持 ${rejection.file.name}，请选择 Markdown、TXT、PDF 或 DOCX`,
      };
    });
    addSelectedFiles(acceptedFiles, rejected);
  }, [addSelectedFiles]);

  const removePendingFile = useCallback((key: string) => {
    const item = filesStateRef.current.files.find((entry) => entry.key === key);
    if (item?.previewUrl) URL.revokeObjectURL(item.previewUrl);
    setPendingFiles((prev) => prev.filter((entry) => entry.key !== key));
  }, []);

  const retryPendingFile = useCallback((key: string) => {
    setPendingFiles((prev) => prev.map((item) =>
      item.key === key
        ? { ...item, status: 'queued', errorMessage: undefined, retryable: false }
        : item));
  }, []);

  function cancelUpload() {
    setCancelled(true);
    uploadControllerRef.current?.abort();
    uploadControllerRef.current = null;
    setUploadProgress(0);
    setNotice('');
    filesStateRef.current.files.forEach((item) => {
      if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
    });
    setPendingFiles([]);
  }

  // 选择文件夹：浏览器允许读取目录内全部文件，前端打包为 zip 上传，
  // 后端按文件名（basename）匹配 Markdown 中的本机绝对路径图片引用。
  // File.webkitRelativePath 由 DOM 库提供（目录选择时携带相对路径）。
  async function selectFolder(selected: File[]) {
    const documents = selected.filter((file) =>
      importableDocumentExtensions.some((extension) => file.name.toLowerCase().endsWith(extension)));
    if (documents.length === 0) {
      setValidationError('文件夹中没有可导入的文档或图片');
      return;
    }
    const oversized = selected.find((file) => file.size <= 0 || file.size > MAX_FILE_SIZE);
    if (oversized) {
      setValidationError(`${oversized.name} 为空或超过 50 MB`);
      return;
    }
    setPacking(true);
    try {
      const zip = new JSZip();
      for (const file of selected) {
        // 去掉所选根目录名，保留目录内相对路径。
        const parts = (file.webkitRelativePath || file.name).split('/');
        const relative = parts.slice(1).join('/') || file.name;
        zip.file(relative, file);
      }
      const blob = await zip.generateAsync({ type: 'blob' });
      const folderName = (selected[0].webkitRelativePath || 'import').split('/')[0] || 'import';
      const zipFile = new File([blob], `${folderName}.zip`, { type: 'application/zip' });
      if (zipFile.size > MAX_FILE_SIZE) {
        setValidationError('文件夹压缩后超过 50 MB，请拆分文件夹后导入');
        return;
      }
      setPendingFiles((prev) => {
        prev.forEach((item) => {
          if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
        });
        return [createPendingFile(zipFile)];
      });
      setFileImportMode('folder_archive');
      setValidationError('');
      setNotice(`已打包 ${documents.length} 个可识别文件和 ${selected.length - documents.length} 个资源/待校验文件为 ${zipFile.name}`);
    } catch {
      setValidationError('打包文件夹失败，请重试');
    } finally {
      setPacking(false);
    }
  }

  const pendingCount = pendingTasks.filter((task) => !task.started).length;
  const busy = uploadMutation.isPending || startMutation.isPending || attachmentMutation.isPending || packing;
  const selectionLocked = disabled || busy || pendingCount > 0;
  const canUpload = sourceMode === 'file' ? queuedFiles.length > 0 : /^https?:\/\//i.test(sourceURL.trim());

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop: onDropFiles,
    accept: uploadAccept,
    maxSize: MAX_FILE_SIZE,
    minSize: 1,
    multiple: true,
    disabled: selectionLocked,
    noClick: false,
    noKeyboard: false,
  });

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
        {uploadMutation.error && !cancelled && <Alert severity="error">{errorMessage(uploadMutation.error)}</Alert>}
        {startMutation.error && <Alert severity="error">开始导入失败：{errorMessage(startMutation.error)}</Alert>}
        {attachmentMutation.error && <Alert severity="error">图片补传失败：{errorMessage(attachmentMutation.error)}</Alert>}
        {(startMutation.isPending || attachmentMutation.isPending || packing) && <LinearProgress />}

        {sourceMode === 'file' ? (
          <>
            <div
              {...getRootProps()}
              role="button"
              tabIndex={0}
              aria-label="拖放文件上传"
              className={clsx(
                'flex cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 px-4 py-8 text-center',
                'transition-colors duration-200 motion-reduce:transition-none',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30',
                isDragActive
                  ? 'border-solid border-blue-500 bg-blue-500/10'
                  : 'border-dashed border-zinc-200 bg-zinc-50 hover:border-blue-500/60',
                selectionLocked && 'pointer-events-none opacity-60',
              )}
            >
              <input {...getInputProps()} />
              <CloudUploadOutlined className={clsx('h-10 w-10', isDragActive ? 'text-blue-500' : 'text-zinc-400')} />
              <p className="text-sm font-medium text-zinc-900">
                {isDragActive ? '松开文件即可上传' : '拖放文件到此處'}
              </p>
              <span className="rounded-lg text-sm font-medium transition-colors duration-200 bg-zinc-100 text-zinc-700 hover:bg-zinc-200 px-3 py-1.5">
                或点击选择文件
              </span>
              <p className="text-xs text-zinc-400">
                支持 Markdown、TXT、PDF、DOCX、XLSX、PPTX、图片（JPG/PNG/BMP/TIFF）、ZIP；单次最多 {MAX_FILES} 个文件，单个最大 50 MB
              </p>
            </div>
            <Stack direction="row" spacing={1}>
              <button
                type="button"
                disabled={selectionLocked || packing}
                onClick={() => folderInputRef.current?.click()}
                className="rounded-lg text-sm font-medium transition-colors duration-200 motion-reduce:transition-none bg-zinc-100 text-zinc-700 hover:bg-zinc-200 active:scale-95 disabled:opacity-50 px-3 py-1.5"
              >
                {packing ? '正在打包…' : '选择文件夹'}
              </button>
            </Stack>
            <input
              ref={folderInputRef}
              type="file"
              multiple
              hidden
              {...({ webkitdirectory: '', directory: '' } as Record<string, string>)}
              onChange={(event) => void selectFolder(Array.from(event.target.files ?? []))}
            />
            {pendingFiles.length > 0 && (
              <ul className="rounded-xl border border-zinc-200 bg-white shadow-sm">
                {pendingFiles.map((item) => (
                  <li key={item.key} className="flex items-center gap-3 border-b border-zinc-200 px-4 py-3 last:border-b-0">
                    {item.previewUrl ? (
                      <img
                        src={item.previewUrl}
                        alt=""
                        className="h-10 w-10 shrink-0 rounded-lg border border-zinc-200 object-cover"
                      />
                    ) : (
                      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-zinc-100 text-zinc-400">
                        <InsertDriveFileOutlined className="h-5 w-5" />
                      </span>
                    )}
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-1.5">
                        <p className="truncate text-sm font-medium text-zinc-900">{item.file.name}</p>
                        {item.status === 'success' && <CheckCircleOutlined className="h-4 w-4 shrink-0 text-emerald-500" />}
                        {item.status === 'error' && <ErrorOutlineOutlined className="h-4 w-4 shrink-0 text-red-500" />}
                      </div>
                      {item.status === 'uploading' ? (
                        <div className="mt-1.5 flex items-center gap-3">
                          <div className="h-1.5 flex-1 overflow-hidden rounded-lg bg-zinc-100">
                            <div
                              className="h-full rounded-lg bg-blue-500 transition-[width] duration-200 motion-reduce:transition-none"
                              style={{ width: `${uploadProgress}%` }}
                            />
                          </div>
                          <span className="shrink-0 text-xs text-zinc-400">{uploadProgress}%</span>
                        </div>
                      ) : (
                        <p className={clsx('mt-1 text-xs', item.errorMessage ? 'text-red-500' : 'text-zinc-400')}>
                          {item.errorMessage ?? `${(item.file.size / 1024).toFixed(1)} KB`}
                        </p>
                      )}
                    </div>
                    <div className="flex shrink-0 items-center gap-1">
                      {item.status === 'uploading' && (
                        <button type="button" title="取消上传" aria-label="取消上传" onClick={cancelUpload} className={iconButtonClass}>
                          <CloseOutlined className="h-4 w-4" />
                        </button>
                      )}
                      {item.status === 'error' && item.retryable && (
                        <button type="button" onClick={() => retryPendingFile(item.key)} className={retryButtonClass}>
                          <ReplayOutlined className="h-4 w-4" />
                          重试
                        </button>
                      )}
                      {item.status !== 'uploading' && (
                        <button
                          type="button"
                          title="删除"
                          aria-label={`删除 ${item.file.name}`}
                          onClick={() => removePendingFile(item.key)}
                          className={iconButtonClass}
                        >
                          <DeleteOutlineOutlined className="h-4 w-4" />
                        </button>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}
            {pendingTasks.length > 0 && (
              <Stack spacing={1.5}>
                {pendingTasks.map((task) => (
                  <Box key={task.taskId} sx={{ border: 1, borderColor: 'divider', borderRadius: 1, p: 1.5 }}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Typography variant="body2" fontWeight={700} sx={{ flexGrow: 1 }}>{task.sourcePath || task.fileName}</Typography>
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
