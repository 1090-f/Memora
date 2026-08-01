import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { computed } from 'vue'
import {
  listMcpServers,
  getMcpServer,
  createMcpServer,
  updateMcpServer,
  deleteMcpServer,
  testMcpServer,
  discoverMcpTools,
  listMcpTools,
  toggleMcpTool,
  grantToolToKnowledgeBase,
  revokeToolFromKnowledgeBase,
} from './api'
import { mcpKeys } from './types'
import type { CreateMcpServerRequest, UpdateMcpServerRequest } from './types'

// Server queries
export function useMcpServerList() {
  return useQuery({
    queryKey: mcpKeys.servers,
    queryFn: listMcpServers,
  })
}

export function useMcpServerDetail(id: () => string) {
  return useQuery({
    queryKey: computed(() => mcpKeys.server(id())),
    queryFn: () => getMcpServer(id()),
    enabled: computed(() => !!id()),
  })
}

export function useCreateMcpServer() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateMcpServerRequest) => createMcpServer(data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: mcpKeys.servers })
    },
  })
}

export function useUpdateMcpServer() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateMcpServerRequest }) =>
      updateMcpServer(id, data),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: mcpKeys.servers })
    },
  })
}

export function useDeleteMcpServer() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteMcpServer(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: mcpKeys.servers })
    },
  })
}

export function useTestMcpServer() {
  return useMutation({
    mutationFn: (id: string) => testMcpServer(id),
  })
}

export function useDiscoverMcpTools() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => discoverMcpTools(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({ queryKey: mcpKeys.tools(id) })
    },
  })
}

// Tool queries
export function useMcpToolList(serverId: () => string) {
  return useQuery({
    queryKey: computed(() => mcpKeys.tools(serverId())),
    queryFn: () => listMcpTools(serverId()),
    enabled: computed(() => !!serverId()),
  })
}

export function useToggleMcpTool(serverId: () => string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ toolId, enabled }: { toolId: string; enabled: boolean }) =>
      toggleMcpTool(toolId, enabled),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: mcpKeys.tools(serverId()) })
    },
  })
}

// Knowledge base tool authorization
export function useGrantTool() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ kbId, toolId }: { kbId: string; toolId: string }) =>
      grantToolToKnowledgeBase(kbId, toolId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['agent-config'] })
    },
  })
}

export function useRevokeTool() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ kbId, toolId }: { kbId: string; toolId: string }) =>
      revokeToolFromKnowledgeBase(kbId, toolId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['agent-config'] })
    },
  })
}
