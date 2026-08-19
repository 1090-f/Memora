import { apiBlobRequest, apiRequest } from '@/api/client';
import type {
  CreateDirectoryInput,
  DirectoryNode,
  Document,
  DocumentListItem,
  DocumentListParams,
  DocumentIndexVersion,
  DocumentReadPage,
  DocumentProcessing,
  DocumentPreview,
  DocumentTablePreview,
  DocumentTextPreview,
  ImageScanResult,
  ImportSubmission,
  ImportUploadResponse,
  ImportTask,
  ImportURLInput,
} from './types';
import type { PageResult } from '@/features/knowledge-base/types';

// 文档、导入、受限正文读取与索引版本接口统一从此模块访问。
export const getDirectoryTree = (kbId: string) =>
  apiRequest<DirectoryNode[]>({ url: `/knowledge-bases/${kbId}/directories/tree` });

export const getSupportedExtensions = () =>
  apiRequest<{ supported_extensions: string[] }>({ url: '/documents/supported-extensions' });

export const createDirectory = (kbId: string, input: CreateDirectoryInput) =>
  apiRequest<DirectoryNode>({ url: `/knowledge-bases/${kbId}/directories`, method: 'POST', data: input });

export const listDocuments = (kbId: string, params: DocumentListParams = {}) =>
  apiRequest<PageResult<DocumentListItem>>({ url: `/knowledge-bases/${kbId}/documents`, params });

export const getDocument = (documentId: string) =>
  apiRequest<Document>({ url: `/documents/${documentId}` });

export const getDocumentPreview = (documentId: string) =>
  apiRequest<DocumentPreview>({ url: `/documents/${documentId}/preview` });

const descriptorPath = (url: string) => url.startsWith('/api/v1/') ? url.slice('/api/v1'.length) : url;

export const getDocumentTextPreview = (contentUrl: string) =>
  apiRequest<DocumentTextPreview>({ url: descriptorPath(contentUrl) });

export const getDocumentTextPreviewById = (documentId: string) =>
  apiRequest<DocumentTextPreview>({ url: `/documents/${documentId}/preview/text` });

export const getDocumentTablePreview = (contentUrl: string, params: { sheet_index: number; row_offset: number; row_limit?: number }) =>
  apiRequest<DocumentTablePreview>({ url: descriptorPath(contentUrl), params });

export const getDocumentPreviewBlob = (contentUrl: string) =>
  apiBlobRequest({ url: descriptorPath(contentUrl) });

export const retryDocumentPreview = (documentId: string) =>
  apiRequest<DocumentPreview>({ url: `/documents/${documentId}/preview/retry`, method: 'POST' });

export const getOriginalDocument = (documentId: string, inline = false) =>
  apiBlobRequest({ url: `/documents/${documentId}/original`, params: inline ? { inline: true } : undefined });

// getRenderedDocument 返回渲染预览 PDF（PDF 原文件 / Office 文档经 LibreOffice 转换）。
// 大文件首次转换可能超过默认 120s，这里单独放宽到 10 分钟。
export const getRenderedDocument = (documentId: string) =>
  apiBlobRequest({ url: `/documents/${documentId}/rendered` });

export const deleteDocument = (documentId: string) =>
  apiRequest<{ deleted: boolean }>({ url: `/documents/${documentId}`, method: 'DELETE' });

export const moveDocument = (documentId: string, directoryId?: string) =>
  apiRequest<{ moved: boolean }>({
    url: `/documents/${documentId}/directory`,
    method: 'PATCH',
    data: { directory_id: directoryId || null },
  });

export const importFiles = (
  kbId: string,
  formData: FormData,
  options: { onUploadProgress?: (percent: number) => void; signal?: AbortSignal } = {},
) =>
  // FormData 交给 Axios 自动生成带 boundary 的 Content-Type，避免手工设置导致后端无法解析。
  apiRequest<ImportUploadResponse>({
    url: `/knowledge-bases/${kbId}/imports/files`, method: 'POST', data: formData,
    signal: options.signal,
    onUploadProgress: options.onUploadProgress
      ? (event) => {
          if (event.total && event.total > 0) {
            options.onUploadProgress?.(Math.round((event.loaded / event.total) * 100));
          }
        }
      : undefined,
  });

export const importURL = (kbId: string, input: ImportURLInput) =>
  apiRequest<ImportSubmission>({ url: `/knowledge-bases/${kbId}/imports/url`, method: 'POST', data: input });

export const listImportTasks = (kbId: string, params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<ImportTask>>({ url: `/knowledge-bases/${kbId}/import-tasks`, params });

export const cleanupImportTasks = (kbId: string) =>
  apiRequest<{ deleted: number }>({ url: `/knowledge-bases/${kbId}/import-tasks`, method: 'DELETE' });

export const getImportTask = (taskId: string) =>
  apiRequest<ImportTask>({ url: `/import-tasks/${taskId}` });

export const retryImportTask = (taskId: string) =>
  apiRequest<{ retried: boolean }>({ url: `/import-tasks/${taskId}/retry`, method: 'POST' });

export const startImportTask = (taskId: string) =>
  apiRequest<{ started: boolean }>({ url: `/import-tasks/${taskId}/start`, method: 'POST' });

export const scanImportTask = (taskId: string) =>
  apiRequest<ImageScanResult>({ url: `/import-tasks/${taskId}/scan`, method: 'POST' });

export const uploadTaskAttachments = (taskId: string, files: File[]) => {
  const formData = new FormData();
  files.forEach((file) => formData.append('files', file));
  return apiRequest<{ uploaded: number }>({ url: `/import-tasks/${taskId}/attachments`, method: 'POST', data: formData });
};

export const getDocumentProcessing = (documentId: string) =>
  apiRequest<DocumentProcessing>({ url: `/documents/${documentId}/processing` });

export const retryDocumentProcessing = (documentId: string) =>
  apiRequest<{ retried: boolean }>({ url: `/documents/${documentId}/retry-processing`, method: 'POST' });

export const reindexDocument = (documentId: string) =>
  apiRequest<{ document_id: string; status: string }>({ url: `/documents/${documentId}/reindex`, method: 'POST' });

export const readDocumentContent = (kbId: string, documentId: string, params: { cursor?: string; section?: string; max_tokens?: number } = {}) =>
  apiRequest<DocumentReadPage>({ url: `/knowledge-bases/${kbId}/documents/${documentId}/content`, params });

export const listDocumentIndexVersions = (documentId: string) =>
  apiRequest<{ items: DocumentIndexVersion[] }>({ url: `/documents/${documentId}/index-versions` });
