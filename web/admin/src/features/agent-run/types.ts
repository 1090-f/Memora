export type AgentRunStatus = 'idle' | 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';
export type AgentEventType =
  | 'run.started' | 'run.completed' | 'run.failed' | 'run.cancelled'
  | 'router.selected' | 'memory.retrieved' | 'plan.created' | 'plan.replanned'
  | 'step.started' | 'step.completed' | 'step.failed' | 'agent.round.started'
  | 'tool.call.started' | 'tool.call.completed' | 'tool.call.failed'
  | 'answer.delta' | 'citation.created' | 'usage.updated' | 'memory.updated';

export interface AgentEvent {
  run_id: string;
  sequence: number;
  timestamp: string;
  type: AgentEventType;
  payload: Record<string, unknown>;
}

export interface AgentRun {
  id: string;
  knowledge_base_id: string;
  conversation_id: string;
  query: string;
  execution_mode: 'react' | 'plan_execute';
  knowledge_status: 'sufficient' | 'insufficient' | 'ambiguous';
  status: Exclude<AgentRunStatus, 'idle'>;
  router_reason_summary: string | null;
  final_result: string | null;
  error_code: string | null;
  error_message: string | null;
  started_at: string | null;
  ended_at: string | null;
}

export interface AgentRunViewState {
  highest_sequence: number;
  status: AgentRunStatus;
  answer: string;
  router: { execution_mode: 'react' | 'plan_execute'; reason_summary: string } | null;
  plan: { version: number; reason_summary?: string; steps: Array<{ step_no: number; title: string; status: string; error_message?: string }> } | null;
  rounds: Array<{ round_no: number; status: string; action_summary: string; tool_name?: string }>;
  tools: Array<{ tool_call_id: string; tool_name: string; status: string; input_summary?: string; output_summary?: string }>;
  citations: Array<Record<string, unknown>>;
  usage: { input_tokens: number; output_tokens: number; total_tokens: number } | null;
  error: { code: string; message: string } | null;
  resumable: boolean;
}
