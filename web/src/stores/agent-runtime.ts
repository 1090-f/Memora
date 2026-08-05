import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type {
  KnownAgentEvent,
  ParsedAgentEvent,
  PlanEventPayload,
  StepEventPayload,
  RoundEventPayload,
  ToolCallEventPayload,
} from '@/features/agent-run/types'
import type { Citation } from '@/features/conversation/types'

export interface RuntimePlanVersion {
  plan_id: string
  version: 1 | 2
  goal: string
  replan_reason?: string
}

export interface RuntimeStep {
  step_id: string
  step_no: number
  title: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  output_summary?: string
  error_message?: string
}

export interface RuntimeRound {
  round_no: number
  action_summary?: string
}

export interface RuntimeToolCall {
  tool_call_id: string
  tool_name: string
  tool_type: 'internal' | 'mcp'
  status: 'running' | 'succeeded' | 'failed'
  input_summary?: string
  output_summary?: string
  duration_ms?: number
  is_truncated?: boolean
  error_message?: string
}

export const useAgentRuntimeStore = defineStore('agent-runtime', () => {
  const run_id = ref<string | null>(null)
  const status = ref<'idle' | 'streaming' | 'completed' | 'failed' | 'cancelled'>('idle')
  const last_sequence = ref<number>(-1)
  const execution_mode = ref<'react' | 'plan_execute' | null>(null)
  const router_reason_summary = ref('')
  const answer = ref('')
  const citations = ref<Citation[]>([])
  const plan_versions = ref<RuntimePlanVersion[]>([])
  const steps = ref<RuntimeStep[]>([])
  const rounds = ref<RuntimeRound[]>([])
  const tool_calls = ref<RuntimeToolCall[]>([])
  const usage = ref({ input_tokens: 0, output_tokens: 0, total_tokens: 0 })
  const error = ref<{ code: string; message: string } | null>(null)
  const memory_count = ref(0)

  const is_active = computed(() => status.value === 'streaming')

  function startRun(id: string) {
    run_id.value = id
    status.value = 'streaming'
    last_sequence.value = -1
    execution_mode.value = null
    router_reason_summary.value = ''
    answer.value = ''
    citations.value = []
    plan_versions.value = []
    steps.value = []
    rounds.value = []
    tool_calls.value = []
    usage.value = { input_tokens: 0, output_tokens: 0, total_tokens: 0 }
    error.value = null
    memory_count.value = 0
  }

  function handleEvent(event: ParsedAgentEvent) {
    if (event.unknown) return
    if (event.sequence <= last_sequence.value) return
    last_sequence.value = event.sequence

    const e = event as KnownAgentEvent

    switch (e.event) {
      case 'run.started':
        status.value = 'streaming'
        break

      case 'run.completed':
        status.value = 'completed'
        break

      case 'run.failed':
        status.value = 'failed'
        error.value = e.payload as { code: string; message: string }
        break

      case 'run.cancelled':
        status.value = 'cancelled'
        break

      case 'router.selected': {
        const p = e.payload as { execution_mode: 'react' | 'plan_execute'; reason_summary: string }
        execution_mode.value = p.execution_mode
        router_reason_summary.value = p.reason_summary
        break
      }

      case 'memory.retrieved':
        memory_count.value = (e.payload as { count: number }).count
        break

      case 'plan.created':
      case 'plan.replanned': {
        const p = e.payload as PlanEventPayload
        const existing = plan_versions.value.find(v => v.version === p.version)
        if (existing) {
          existing.goal = p.goal
          existing.replan_reason = p.replan_reason
        } else {
          plan_versions.value.push({
            plan_id: p.plan_id,
            version: p.version,
            goal: p.goal,
            replan_reason: p.replan_reason,
          })
        }
        break
      }

      case 'step.started': {
        const p = e.payload as StepEventPayload
        const existing = steps.value.find(s => s.step_id === p.step_id)
        if (existing) {
          existing.title = p.title
          existing.status = 'running'
        } else {
          steps.value.push({
            step_id: p.step_id,
            step_no: p.step_no,
            title: p.title,
            status: 'running',
          })
        }
        break
      }

      case 'step.completed': {
        const p = e.payload as StepEventPayload
        const existing = steps.value.find(s => s.step_id === p.step_id)
        if (existing) {
          existing.status = 'completed'
          existing.output_summary = p.output_summary
        }
        break
      }

      case 'step.failed': {
        const p = e.payload as StepEventPayload
        const existing = steps.value.find(s => s.step_id === p.step_id)
        if (existing) {
          existing.status = 'failed'
          existing.error_message = p.error_message
        }
        break
      }

      case 'agent.round.started': {
        const p = e.payload as RoundEventPayload
        rounds.value.push({
          round_no: p.round_no,
          action_summary: p.action_summary,
        })
        break
      }

      case 'tool.call.started': {
        const p = e.payload as ToolCallEventPayload
        const existing = tool_calls.value.find(t => t.tool_call_id === p.tool_call_id)
        if (existing) {
          existing.status = 'running'
        } else {
          tool_calls.value.push({
            tool_call_id: p.tool_call_id,
            tool_name: p.tool_name,
            tool_type: p.tool_type,
            status: 'running',
            input_summary: p.input_summary,
          })
        }
        break
      }

      case 'tool.call.completed': {
        const p = e.payload as ToolCallEventPayload
        const existing = tool_calls.value.find(t => t.tool_call_id === p.tool_call_id)
        if (existing) {
          existing.status = 'succeeded'
          existing.output_summary = p.output_summary
          existing.duration_ms = p.duration_ms
          existing.is_truncated = p.is_truncated
        }
        break
      }

      case 'tool.call.failed': {
        const p = e.payload as ToolCallEventPayload
        const existing = tool_calls.value.find(t => t.tool_call_id === p.tool_call_id)
        if (existing) {
          existing.status = 'failed'
          existing.error_message = p.error_message
          existing.duration_ms = p.duration_ms
        }
        break
      }

      case 'answer.delta': {
        const p = e.payload as { delta: string }
        answer.value += p.delta
        break
      }

      case 'citation.created': {
        const p = e.payload as { citation: Citation }
        citations.value.push(p.citation)
        break
      }

      case 'usage.updated': {
        const p = e.payload as { input_tokens: number; output_tokens: number; total_tokens: number }
        usage.value = p
        break
      }
    }
  }

  function reset() {
    run_id.value = null
    status.value = 'idle'
    last_sequence.value = -1
    execution_mode.value = null
    router_reason_summary.value = ''
    answer.value = ''
    citations.value = []
    plan_versions.value = []
    steps.value = []
    rounds.value = []
    tool_calls.value = []
    usage.value = { input_tokens: 0, output_tokens: 0, total_tokens: 0 }
    error.value = null
    memory_count.value = 0
  }

  return {
    run_id,
    status,
    last_sequence,
    execution_mode,
    router_reason_summary,
    answer,
    citations,
    plan_versions,
    steps,
    rounds,
    tool_calls,
    usage,
    error,
    memory_count,
    is_active,
    startRun,
    handleEvent,
    reset,
  }
})
