import { request } from '@/api/client'
import { toSearchParams } from '@/api/pagination'
import type { PageData, PageQuery } from '@/types/common'
import type { Citation } from '@/features/conversation/types'

export interface AgentRun {
  id: string
  knowledge_base_id: string
  conversation_id: string
  query: string
  execution_mode: 'react' | 'plan_execute' | null
  knowledge_status: 'sufficient' | 'insufficient' | 'ambiguous' | null
  status: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'
  router_reason_summary: string | null
  memory_used_count: number | null
  input_tokens: number | null
  output_tokens: number | null
  total_tokens: number | null
  duration_ms: number | null
  final_result: string | null
  error_code: string | null
  error_message: string | null
  started_at: string | null
  ended_at: string | null
  created_at: string
}

export interface RouterDecision {
  execution_mode: 'react' | 'plan_execute'
  reason_summary: string
  confidence: number
  created_at: string
}

export interface PlanVersion {
  id: string
  version: number
  goal: string
  status: string
  is_current: boolean
  replan_reason?: string
  steps: PlanStep[]
}

export interface PlanStep {
  id: string
  step_no: number
  title: string
  depends_on: string[]
  recommended_tool: string | null
  status: string
  output_summary: string | null
}

export interface ReActRound {
  round_no: number
  status: string
  action_summary: string | null
  tool_name: string | null
  duration_ms: number | null
  token_count: number | null
}

export interface ToolCall {
  tool_call_id: string
  tool_name: string
  tool_type: 'internal' | 'mcp'
  mcp_server_id: string | null
  mcp_tool_id: string | null
  status: string
  input_summary: string | null
  output_summary: string | null
  duration_ms: number | null
  is_truncated: boolean | null
  created_at: string
}

export interface AgentRunListQuery extends PageQuery {
  status?: string
  knowledge_base_id?: string
  execution_mode?: string
}

export function listAgentRuns(query?: AgentRunListQuery): Promise<PageData<AgentRun>> {
  const params = toSearchParams(query)
  if (query?.status) params.set('status', query.status)
  if (query?.knowledge_base_id) params.set('knowledge_base_id', query.knowledge_base_id)
  if (query?.execution_mode) params.set('execution_mode', query.execution_mode)
  return request<PageData<AgentRun>>(`/agent-runs?${params.toString()}`)
}

export function getAgentRun(id: string): Promise<AgentRun> {
  return request<AgentRun>(`/agent-runs/${id}`)
}

export function getRouterDecision(id: string): Promise<RouterDecision> {
  return request<RouterDecision>(`/agent-runs/${id}/router-decision`)
}

export function getPlans(id: string): Promise<PlanVersion[]> {
  return request<PlanVersion[]>(`/agent-runs/${id}/plans`)
}

export function getRounds(id: string): Promise<ReActRound[]> {
  return request<ReActRound[]>(`/agent-runs/${id}/rounds`)
}

export function getToolCalls(id: string): Promise<ToolCall[]> {
  return request<ToolCall[]>(`/agent-runs/${id}/tool-calls`)
}

export function getRunCitations(id: string): Promise<Citation[]> {
  return request<Citation[]>(`/agent-runs/${id}/citations`)
}
