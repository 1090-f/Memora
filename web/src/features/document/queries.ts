import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import {
  getDirectoryTree,
  createDirectory,
  updateDirectory,
  deleteDirectory,
  listDocuments,
  getDocument,
  deleteDocument,
  getProcessingState,
  retryProcessing,
  reindexDocument,
  getIndexVersions,
  importFiles,
  importUrl,
  listImportTasks,
  getImportTask,
  retryImportTask,
} from './api'
import { documentKeys } from './types'
import type { CreateDirectoryRequest, UpdateDirectoryRequest, DocumentListQuery } from './types'

// Directory queries
export function useDirectoryTree(kbId: () => string) {
  return useQuery({
    queryKey: computed(() => documentKeys.directories(kbId())),
    queryFn: () => getDirectoryTree(kbId()),
    enabled: computed(() => !!kbId()),
  })
}

export function useCreateDirectory(kbId: () => string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateDirectoryRequest) => createDirectory(kbId(), data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: documentKeys.directories(kbId()) })
    },
  })
}

export function useUpdateDirectory() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateDirectoryRequest }) =>
      updateDirectory(id, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['directories'] })
    },
  })
}

export function useDeleteDirectory() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteDirectory(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['directories'] })
    },
  })
}

// Document queries
export function useDocumentList(kbId: () => string, query: () => DocumentListQuery) {
  return useQuery({
    queryKey: computed(() => documentKeys.list(kbId(), query())),
    queryFn: () => listDocuments(kbId(), query()),
    enabled: computed(() => !!kbId()),
  })
}

export function useDocumentDetail(id: () => string) {
  return useQuery({
    queryKey: computed(() => documentKeys.detail(id())),
    queryFn: () => getDocument(id()),
    enabled: computed(() => !!id()),
  })
}

export function useDeleteDocument() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteDocument(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: documentKeys.all })
    },
  })
}

export function useProcessingState(id: () => string) {
  return useQuery({
    queryKey: computed(() => documentKeys.processing(id())),
    queryFn: () => getProcessingState(id()),
    enabled: computed(() => !!id()),
    refetchInterval: (query) => {
      const status = query.state.data?.processing_status
      if (status === 'succeeded' || status === 'failed') return false
      return 3000
    },
  })
}

export function useRetryProcessing() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => retryProcessing(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({ queryKey: documentKeys.processing(id) })
      void queryClient.invalidateQueries({ queryKey: documentKeys.all })
    },
  })
}

export function useReindexDocument() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason?: string }) => reindexDocument(id, reason),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: documentKeys.processing(variables.id) })
    },
  })
}

export function useIndexVersions(id: () => string) {
  return useQuery({
    queryKey: computed(() => [...documentKeys.detail(id()), 'index-versions']),
    queryFn: () => getIndexVersions(id()),
    enabled: computed(() => !!id()),
  })
}

// Import queries
export const importTaskKeys = {
  all: ['import-tasks'] as const,
  list: (kbId: string) => ['import-tasks', 'list', kbId] as const,
  detail: (taskId: string) => ['import-tasks', 'detail', taskId] as const,
}

export function useImportFileMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ kbId, formData }: { kbId: string; formData: FormData }) =>
      importFiles(kbId, formData),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: importTaskKeys.list(variables.kbId) })
      void queryClient.invalidateQueries({ queryKey: documentKeys.all })
    },
  })
}

export function useImportUrlMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ kbId, data }: { kbId: string; data: { url: string; directory_id?: string; duplicate_policy?: string } }) =>
      importUrl(kbId, data),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: importTaskKeys.list(variables.kbId) })
      void queryClient.invalidateQueries({ queryKey: documentKeys.all })
    },
  })
}

export function useImportTaskList(kbId: () => string) {
  return useQuery({
    queryKey: computed(() => importTaskKeys.list(kbId())),
    queryFn: () => listImportTasks(kbId()),
    enabled: computed(() => !!kbId()),
  })
}

export function useImportTaskDetail(taskId: () => string) {
  return useQuery({
    queryKey: computed(() => importTaskKeys.detail(taskId())),
    queryFn: () => getImportTask(taskId()),
    enabled: computed(() => !!taskId()),
  })
}

export function useRetryImportTaskMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (taskId: string) => retryImportTask(taskId),
    onSuccess: (_data, taskId) => {
      void queryClient.invalidateQueries({ queryKey: importTaskKeys.detail(taskId) })
      void queryClient.invalidateQueries({ queryKey: ['import-tasks'] })
    },
  })
}
