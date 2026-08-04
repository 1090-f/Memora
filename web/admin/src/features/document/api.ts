import { apiRequest } from '@/api/client';
import type { DirectoryNode, Document, ImportTask } from './types';
import type { PageResult } from '@/features/knowledge-base/types';

export const getDirectoryTree = (kbId: string) =>
  apiRequest<DirectoryNode[]>({ url: `/knowledge-bases/${kbId}/directories/tree` });

export const listDocuments = (kbId: string, params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<Document>>({ url: `/knowledge-bases/${kbId}/documents`, params });

export const getDocument = (documentId: string) =>
  apiRequest<Document>({ url: `/documents/${documentId}` });

export const importFiles = (kbId: string, formData: FormData) =>
  apiRequest<{ tasks: Array<{ task_id: string; file_name: string; status: ImportTask['status'] }> }>({
    url: `/knowledge-bases/${kbId}/imports/files`, method: 'POST', data: formData,
  });

export const importUrl = (kbId: string, input: { url: string; directory_id?: string; duplicate_policy?: 'create_new' | 'skip' }) =>
  apiRequest<{ tasks: Array<{ task_id: string; status: ImportTask['status'] }> }>({
    url: `/knowledge-bases/${kbId}/imports/url`, method: 'POST', data: input,
  });

export const getImportTask = (taskId: string) =>
  apiRequest<ImportTask>({ url: `/import-tasks/${taskId}` });
