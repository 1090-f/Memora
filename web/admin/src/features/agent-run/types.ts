export type AgentRunStatus =
  | 'idle'
  | 'queued'
  | 'running'
  | 'completed'
  | 'failed'
  | 'cancelled';
export type AgentEventType =
  | 'agent.run.queued'
  | 'agent.run.started'
  | 'agent.run.completed'
  | 'agent.run.failed'
  | 'agent.run.cancelled'
  | 'agent.router.completed'
  | 'agent.plan.created'
  | 'agent.plan.step.started'
  | 'agent.plan.step.completed'
  | 'agent.plan.replanned'
  | 'agent.review.completed'
  | 'agent.step.started'
  | 'agent.step.completed'
  | 'agent.react.round.started'
  | 'agent.react.round.completed'
  | 'agent.tool.started'
  | 'agent.tool.completed'
  | 'agent.tool.call.failed'
  | 'agent.answer.delta'
  | 'agent.stage.updated'
  | 'citation.created'
  | 'usage.updated'
  | 'memory.updated';

export interface AgentEvent {
  run_id: string;
  sequence: number;
  timestamp: string;
  type: AgentEventType;
  stage?: string;
  status?: string;
  payload: Record<string, unknown>;
}

export interface AgentRunListItem {
  id: string;
  conversation_id: string;
  query: string;
  trace_id?: string | null;
  request_id?: string | null;
  execution_trace?: unknown;
  router_confidence?: number | null;
  router_fallback_used?: boolean;
  execution_mode?: 'react' | 'plan_execute';
  status: Exclude<AgentRunStatus, 'idle'>;
  total_tokens: number;
  duration_ms?: number;
  error_code?: string;
  created_at: string;
}

export interface AgentRun {
  id: string;
  user_id?: string;
  knowledge_base_id?: string;
  conversation_id?: string;
  agent_config_id?: string;
  chat_model_id?: string;
  retry_of_run_id?: string;
  query: string;
  trace_id?: string | null;
  request_id?: string | null;
  execution_trace?: unknown;
  router_confidence?: number | null;
  router_fallback_used?: boolean;
  execution_mode?: 'react' | 'plan_execute' | null;
  knowledge_status?: 'sufficient' | 'insufficient' | 'ambiguous' | null;
  status: Exclude<AgentRunStatus, 'idle'>;
  router_reason?: string;
  router_reason_summary?: string | null;
  reviewer_result?: string | null;
  final_result?: string | null;
  error_code?: string | null;
  error_message?: string | null;
  replan_count?: number;
  memory_used_count?: number;
  input_tokens?: number;
  output_tokens?: number;
  total_tokens?: number;
  duration_ms?: number | null;
  first_token_at?: string | null;
  first_token_latency_ms?: number | null;
  model_generate_duration_ms?: number | null;
  failure_stage?: string | null;
  retryable?: boolean | null;
  recovery_advice?: string | null;
  started_at?: string | null;
  ended_at?: string | null;
  created_at?: string;
}

export interface CreateAgentRunResponse {
  run_id: string;
  conversation_id: string;
  status: 'queued';
}

// AgentTimelineEntry 表示执行链路中的一条记录。
// reducer 按事件 sequence 顺序追加，started/completed 合并为同一条目，
// 渲染端按数组顺序严格还原真实执行顺序（工具与步骤/轮次穿插展示）。
export type AgentTimelineEntry =
  | { kind: 'status'; sequence: number; title: string; status: string; error_message?: string }
  | { kind: 'stage'; sequence: number; stage: string; status: 'pending' | 'running' | 'succeeded' | 'failed' | 'skipped'; summary?: string; started_at?: string; ended_at?: string; duration_ms?: number; error_code?: string; error_message?: string; metadata?: Record<string, unknown> }
  | { kind: 'router'; sequence: number; execution_mode: 'react' | 'plan_execute'; reason_summary: string; input_summary?: string; confidence?: number; fallback_used?: boolean }
  | { kind: 'plan_created'; sequence: number; version: number; goal: string; step_count: number; replanned?: boolean; input_summary?: string; steps_detail?: Array<{ step_no: number; title: string; description?: string; recommended_tool?: string; depends_on?: string[]; status: string }> }
  | { kind: 'plan_step'; sequence: number; version: number; step_no: number; title: string; status: string; error_message?: string; input_summary?: string; output_summary?: string; duration_ms?: number; token_usage?: { input_tokens: number; output_tokens: number; total_tokens: number } }
  | { kind: 'round'; sequence: number; round_no: number; status: string; action_summary: string; input_summary?: string; model_decision?: string; output_summary?: string; duration_ms?: number; token_usage?: { input_tokens: number; output_tokens: number; total_tokens: number } }
  | { kind: 'tool'; sequence: number; tool_call_id: string; tool_name: string; status: string; input_summary?: string; output_summary?: string; error_message?: string }
  | { kind: 'citation'; sequence: number; citation: Record<string, unknown> }
  | { kind: 'answer'; sequence: number; delta: string };

export interface AgentRunViewState {
  highest_sequence: number;
  status: AgentRunStatus;
  timeline: AgentTimelineEntry[];
  answer: string;
  router: {
    execution_mode: 'react' | 'plan_execute';
    reason_summary: string;
  } | null;
  plan: {
    version: number;
    reason_summary?: string;
    steps: Array<{
      step_no: number;
      title: string;
      status: string;
      error_message?: string;
    }>;
  } | null;
  rounds: Array<{
    round_no: number;
    status: string;
    action_summary: string;
    tool_name?: string;
  }>;
  tools: Array<{
    tool_call_id: string;
    tool_name: string;
    status: string;
    input_summary?: string;
    output_summary?: string;
  }>;
  citations: Array<Record<string, unknown>>;
  usage: {
    input_tokens: number;
    output_tokens: number;
    total_tokens: number;
  } | null;
  error: { code: string; message: string } | null;
  resumable: boolean;
}

export interface AgentToolCall {
  id: string;
  tool_name: string;
  tool_type: string;
  status: 'running' | 'succeeded' | 'failed' | 'timeout' | 'cancelled';
  react_round_no?: number;
  input_summary?: string;
  output_summary?: string;
  arguments_redacted?: string | null;
  result_meta?: unknown;
  response_bytes?: number | null;
  is_truncated?: boolean;
  error_code?: string | null;
  error_message?: string | null;
  duration_ms?: number | null;
  started_at: string;
  ended_at?: string | null;
}

export interface TraceSpan {
  trace_id: string;
  span_id: string;
  parent_span_id?: string | null;
  name: string;
  kind: string;
  status_code: 'Unset' | 'Ok' | 'Error' | string;
  status_message?: string | null;
  started_at: string;
  ended_at: string;
  duration_ms: number;
  attributes: Record<string, unknown>;
  events: Array<Record<string, unknown>>;
  service_name?: string | null;
  instrumentation_scope?: string | null;
}

export type AgentToolCallStatus = AgentToolCall['status'];
