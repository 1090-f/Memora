import { z } from 'zod'
import type {
  KnownAgentEvent,
  UnknownAgentEvent,
  ParsedAgentEvent,
} from './types'

const baseEventSchema = z.object({
  run_id: z.string().uuid(),
  sequence: z.number().int().nonnegative(),
  timestamp: z.string(),
  payload: z.record(z.string(), z.unknown()),
})

const recordPayload = z.record(z.string(), z.unknown())

const planPayload = z.object({
  plan_id: z.string().uuid(),
  version: z.union([z.literal(1), z.literal(2)]),
  goal: z.string(),
  replan_reason: z.string().optional(),
})

const stepPayload = z.object({
  step_id: z.string().uuid(),
  step_no: z.number().int().positive(),
  title: z.string(),
  output_summary: z.string().optional(),
  error_message: z.string().optional(),
})

const roundPayload = z.object({
  round_no: z.number().int().positive(),
  action_summary: z.string().optional(),
})

const toolPayload = z.object({
  tool_call_id: z.string().uuid(),
  tool_name: z.string(),
  tool_type: z.enum(['internal', 'mcp']),
  input_summary: z.string().optional(),
  output_summary: z.string().optional(),
  duration_ms: z.number().nonnegative().optional(),
  is_truncated: z.boolean().optional(),
  error_message: z.string().optional(),
})

const knowledgeCitation = z.object({
  source_type: z.literal('knowledge_base'),
  document_id: z.string().uuid(),
  document_title: z.string(),
  chunk_id: z.string().uuid(),
  quoted_text: z.string(),
  knowledge_base_id: z.string().uuid(),
  source_location: z.object({
    section: z.string().optional(),
    page: z.number().int().positive().optional(),
  }),
  document_updated_at: z.string(),
})

const networkCitation = z.object({
  source_type: z.literal('network'),
  title: z.string(),
  url: z.string().url(),
  site_name: z.string(),
  published_at: z.string().nullable(),
  fetched_at: z.string(),
})

const knownPayloadSchemas = {
  'run.started': recordPayload,
  'run.completed': recordPayload,
  'run.failed': z.object({ code: z.string(), message: z.string() }),
  'run.cancelled': recordPayload,
  'router.selected': z.object({
    execution_mode: z.enum(['react', 'plan_execute']),
    reason_summary: z.string(),
  }),
  'memory.retrieved': z.object({ count: z.number().int().nonnegative() }),
  'plan.created': planPayload,
  'plan.replanned': planPayload,
  'step.started': stepPayload,
  'step.completed': stepPayload,
  'step.failed': stepPayload,
  'agent.round.started': roundPayload,
  'tool.call.started': toolPayload,
  'tool.call.completed': toolPayload,
  'tool.call.failed': toolPayload,
  'answer.delta': z.object({ delta: z.string() }),
  'citation.created': z.object({
    citation: z.union([knowledgeCitation, networkCitation]),
  }),
  'usage.updated': z.object({
    input_tokens: z.number().int().nonnegative(),
    output_tokens: z.number().int().nonnegative(),
    total_tokens: z.number().int().nonnegative(),
  }),
  'memory.updated': z.object({
    memory_id: z.string().uuid().optional(),
    action: z.enum(['created', 'merged', 'updated', 'invalidated']),
  }),
} as const

export function parseAgentEvent(eventName: string, raw: unknown): ParsedAgentEvent {
  const base = baseEventSchema.parse(raw)
  const parser = knownPayloadSchemas[eventName as keyof typeof knownPayloadSchemas]
  if (!parser) {
    return { ...base, event: eventName, unknown: true } as UnknownAgentEvent
  }
  return { ...base, event: eventName, payload: parser.parse(base.payload) } as KnownAgentEvent
}

export function parseSseBlock(block: string): ParsedAgentEvent | null {
  let eventName = 'message'
  const dataLines: string[] = []

  for (const line of block.split('\n')) {
    if (!line || line.startsWith(':')) continue
    const separator = line.indexOf(':')
    const field = separator < 0 ? line : line.slice(0, separator)
    const rawValue = separator < 0 ? '' : line.slice(separator + 1)
    const value = rawValue.startsWith(' ') ? rawValue.slice(1) : rawValue

    if (field === 'event') eventName = value
    if (field === 'data') dataLines.push(value)
  }

  if (dataLines.length === 0) return null

  try {
    return parseAgentEvent(eventName, JSON.parse(dataLines.join('\n')))
  } catch {
    return null
  }
}
