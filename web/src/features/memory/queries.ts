import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import { listMemories, getMemory, updateMemoryStatus, deleteMemory } from './api'
import { memoryKeys } from './types'
import type { MemoryListQuery } from './types'

export function useMemoryList(query: () => MemoryListQuery) {
  return useQuery({
    queryKey: computed(() => memoryKeys.list(query())),
    queryFn: () => listMemories(query()),
  })
}

export function useMemoryDetail(id: () => string) {
  return useQuery({
    queryKey: computed(() => memoryKeys.detail(id())),
    queryFn: () => getMemory(id()),
    enabled: computed(() => !!id()),
  })
}

export function useUpdateMemoryStatus() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: 'active' | 'inactive' }) =>
      updateMemoryStatus(id, status),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: memoryKeys.all })
    },
  })
}

export function useDeleteMemory() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteMemory(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: memoryKeys.all })
    },
  })
}
