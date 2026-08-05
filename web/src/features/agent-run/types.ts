import type { Citation } from '@/features/conversation/types'

export interface AgentEventBase<TName extends string, TPayload> {
  event: TName
  run_id: string
  sequence: number
  timestamp: string
  payload: TPayload
}

export interface PlanEventPayload {
  plan_id: string
  version: 1 | 2
  goal: string
  replan_reason?: string
}

export interface StepEventPayload {
  step_id: string
  step_no: number
  title: string
  output_summary?: string
  error_message?: string
}

export interface RoundEventPayload {
  round_no: number
  action_summary?: string
}

export interface ToolCallEventPayload {
  tool_call_id: string
  tool_name: string
  tool_type: 'internal' | 'mcp'
  input_summary?: string
  output_summary?: string
  duration_ms?: number
  is_truncated?: boolean
  error_message?: string
}

export interface MemoryUpdatedPayload {
  memory_id?: string
  action: 'created' | 'merged' | 'updated' | 'invalidated'
}

export type KnownAgentEvent =
  | AgentEventBase<'run.started', Record<string, never>>
  | AgentEventBase<'run.completed', Record<string, unknown>>
  | AgentEventBase<'run.failed', { code: string; message: string }>
  | AgentEventBase<'run.cancelled', Record<string, never>>
  | AgentEventBase<'router.selected', { execution_mode: 'react' | 'plan_execute'; reason_summary: string }>
  | AgentEventBase<'memory.retrieved', { count: number }>
  | AgentEventBase<'plan.created', PlanEventPayload>
  | AgentEventBase<'plan.replanned', PlanEventPayload>
  | AgentEventBase<'step.started', StepEventPayload>
  | AgentEventBase<'step.completed', StepEventPayload>
  | AgentEventBase<'step.failed', StepEventPayload>
  | AgentEventBase<'agent.round.started', RoundEventPayload>
  | AgentEventBase<'tool.call.started', ToolCallEventPayload>
  | AgentEventBase<'tool.call.completed', ToolCallEventPayload>
  | AgentEventBase<'tool.call.failed', ToolCallEventPayload>
  | AgentEventBase<'answer.delta', { delta: string }>
  | AgentEventBase<'citation.created', { citation: Citation }>
  | AgentEventBase<'usage.updated', { input_tokens: number; output_tokens: number; total_tokens: number }>
  | AgentEventBase<'memory.updated', MemoryUpdatedPayload>

export interface UnknownAgentEvent extends AgentEventBase<string, Record<string, unknown>> {
  unknown: true
}

export type ParsedAgentEvent = KnownAgentEvent | UnknownAgentEvent
