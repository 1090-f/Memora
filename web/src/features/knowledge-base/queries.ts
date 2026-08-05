import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import { listKnowledgeBases, getKnowledgeBase, createKnowledgeBase, updateKnowledgeBase, deleteKnowledgeBase } from './api'
import { knowledgeBaseKeys } from './types'
import type { CreateKnowledgeBaseRequest, UpdateKnowledgeBaseRequest } from './types'
import type { PageQuery } from '@/types/common'

export function useKnowledgeBaseList(query: () => PageQuery) {
  return useQuery({
    queryKey: computed(() => knowledgeBaseKeys.list(query())),
    queryFn: () => listKnowledgeBases(query()),
  })
}

export function useKnowledgeBaseDetail(id: () => string) {
  return useQuery({
    queryKey: computed(() => knowledgeBaseKeys.detail(id())),
    queryFn: () => getKnowledgeBase(id()),
    enabled: computed(() => !!id()),
  })
}

export function useCreateKnowledgeBase() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateKnowledgeBaseRequest) => createKnowledgeBase(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: knowledgeBaseKeys.all })
    },
  })
}

export function useUpdateKnowledgeBase() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateKnowledgeBaseRequest }) =>
      updateKnowledgeBase(id, data),
    onSuccess: (_result, variables) => {
      void queryClient.invalidateQueries({ queryKey: knowledgeBaseKeys.all })
      void queryClient.invalidateQueries({ queryKey: knowledgeBaseKeys.detail(variables.id) })
    },
  })
}

export function useDeleteKnowledgeBase() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteKnowledgeBase(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: knowledgeBaseKeys.all })
    },
  })
}
