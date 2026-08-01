import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import {
  listAgentRuns,
  getAgentRun,
  getRouterDecision,
  getPlans,
  getRounds,
  getToolCalls,
  getRunCitations,
} from './api'
import type { AgentRunListQuery } from './types'

export const agentRunKeys = {
  all: ['agent-runs'] as const,
  list: (query?: AgentRunListQuery) => ['agent-runs', 'list', query] as const,
  detail: (id: string) => ['agent-runs', 'detail', id] as const,
  routerDecision: (id: string) => ['agent-runs', id, 'router-decision'] as const,
  plans: (id: string) => ['agent-runs', id, 'plans'] as const,
  rounds: (id: string) => ['agent-runs', id, 'rounds'] as const,
  toolCalls: (id: string) => ['agent-runs', id, 'tool-calls'] as const,
  citations: (id: string) => ['agent-runs', id, 'citations'] as const,
}

export function useAgentRunList(query: () => AgentRunListQuery) {
  return useQuery({
    queryKey: computed(() => agentRunKeys.list(query())),
    queryFn: () => listAgentRuns(query()),
  })
}

export function useAgentRunDetail(id: () => string) {
  return useQuery({
    queryKey: computed(() => agentRunKeys.detail(id())),
    queryFn: () => getAgentRun(id()),
    enabled: computed(() => !!id()),
  })
}

export function useRouterDecision(id: () => string) {
  return useQuery({
    queryKey: computed(() => agentRunKeys.routerDecision(id())),
    queryFn: () => getRouterDecision(id()),
    enabled: computed(() => !!id()),
  })
}

export function usePlans(id: () => string) {
  return useQuery({
    queryKey: computed(() => agentRunKeys.plans(id())),
    queryFn: () => getPlans(id()),
    enabled: computed(() => !!id()),
  })
}

export function useRounds(id: () => string) {
  return useQuery({
    queryKey: computed(() => agentRunKeys.rounds(id())),
    queryFn: () => getRounds(id()),
    enabled: computed(() => !!id()),
  })
}

export function useToolCalls(id: () => string) {
  return useQuery({
    queryKey: computed(() => agentRunKeys.toolCalls(id())),
    queryFn: () => getToolCalls(id()),
    enabled: computed(() => !!id()),
  })
}

export function useRunCitations(id: () => string) {
  return useQuery({
    queryKey: computed(() => agentRunKeys.citations(id())),
    queryFn: () => getRunCitations(id()),
    enabled: computed(() => !!id()),
  })
}
