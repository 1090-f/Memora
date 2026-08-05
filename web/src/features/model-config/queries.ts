import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import {
  listModelConfigs,
  getModelConfig,
  createModelConfig,
  updateModelConfig,
  deleteModelConfig,
  getSearchConfig,
  updateSearchConfig,
  getAgentConfig,
  updateAgentConfig,
} from './api'
import { modelConfigKeys, searchConfigKeys, agentConfigKeys } from './types'
import type { CreateModelConfigRequest, UpdateModelConfigRequest, SearchConfig, AgentConfig } from './types'

// Model Config queries
export function useModelConfigList() {
  return useQuery({
    queryKey: modelConfigKeys.list,
    queryFn: listModelConfigs,
  })
}

export function useModelConfigDetail(id: () => string) {
  return useQuery({
    queryKey: computed(() => modelConfigKeys.detail(id())),
    queryFn: () => getModelConfig(id()),
    enabled: computed(() => !!id()),
  })
}

export function useCreateModelConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateModelConfigRequest) => createModelConfig(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: modelConfigKeys.all })
    },
  })
}

export function useUpdateModelConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateModelConfigRequest }) =>
      updateModelConfig(id, data),
    onSuccess: (_result, variables) => {
      void queryClient.invalidateQueries({ queryKey: modelConfigKeys.all })
      void queryClient.invalidateQueries({ queryKey: modelConfigKeys.detail(variables.id) })
    },
  })
}

export function useDeleteModelConfig() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteModelConfig(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: modelConfigKeys.all })
    },
  })
}

// Search Config queries
export function useSearchConfig(kbId: () => string) {
  return useQuery({
    queryKey: computed(() => searchConfigKeys.detail(kbId())),
    queryFn: () => getSearchConfig(kbId()),
    enabled: computed(() => !!kbId()),
  })
}

export function useUpdateSearchConfig(kbId: () => string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<SearchConfig>) => updateSearchConfig(kbId(), data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: searchConfigKeys.detail(kbId()) })
    },
  })
}

// Agent Config queries
export function useAgentConfig(kbId: () => string) {
  return useQuery({
    queryKey: computed(() => agentConfigKeys.detail(kbId())),
    queryFn: () => getAgentConfig(kbId()),
    enabled: computed(() => !!kbId()),
  })
}

export function useUpdateAgentConfig(kbId: () => string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: Partial<AgentConfig>) => updateAgentConfig(kbId(), data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: agentConfigKeys.detail(kbId()) })
    },
  })
}
