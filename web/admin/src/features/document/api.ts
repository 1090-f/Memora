import { apiRequest } from '@/api/client';
import type { CreateDirectoryInput, DirectoryNode, Document, DocumentListItem, DocumentProcessing, ImportTask } from './types';
import type { PageResult } from '@/features/knowledge-base/types';

export const getDirectoryTree = (kbId: string) =>
  apiRequest<DirectoryNode[]>({ url: `/knowledge-bases/${kbId}/directories/tree` });

export const createDirectory = (kbId: string, input: CreateDirectoryInput) =>
  apiRequest<DirectoryNode>({ url: `/knowledge-bases/${kbId}/directories`, method: 'POST', data: input });

export const listDocuments = (kbId: string, params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<DocumentListItem>>({ url: `/knowledge-bases/${kbId}/documents`, params });

export const getDocument = (documentId: string) =>
  apiRequest<Document>({ url: `/documents/${documentId}` });

export const deleteDocument = (documentId: string) =>
  apiRequest<{ deleted: boolean }>({ url: `/documents/${documentId}`, method: 'DELETE' });

export const importFiles = (kbId: string, formData: FormData) =>
  apiRequest<{ tasks: Array<{ task_id: string; file_name: string; status: ImportTask['status'] }> }>({
    url: `/knowledge-bases/${kbId}/imports/files`, method: 'POST', data: formData,
  });

export const importUrl = (kbId: string, input: { url: string; directory_id?: string; duplicate_policy?: 'create_new' | 'skip' }) =>
  apiRequest<{ tasks: Array<{ task_id: string; status: ImportTask['status'] }> }>({
    url: `/knowledge-bases/${kbId}/imports/url`, method: 'POST', data: input,
  });

export const listImportTasks = (kbId: string, params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<ImportTask>>({ url: `/knowledge-bases/${kbId}/import-tasks`, params });

export const getImportTask = (taskId: string) =>
  apiRequest<ImportTask>({ url: `/import-tasks/${taskId}` });

export const retryImportTask = (taskId: string) =>
  apiRequest<{ retried: boolean }>({ url: `/import-tasks/${taskId}/retry`, method: 'POST' });

export const getDocumentProcessing = (documentId: string) =>
  apiRequest<DocumentProcessing>({ url: `/documents/${documentId}/processing` });

export const retryDocumentProcessing = (documentId: string) =>
  apiRequest<{ retried: boolean }>({ url: `/documents/${documentId}/retry-processing`, method: 'POST' });

export const reindexDocument = (documentId: string) =>
  apiRequest<{ document_id: string; status: string }>({ url: `/documents/${documentId}/reindex`, method: 'POST' });
