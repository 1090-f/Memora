import type { AgentEvent, AgentRunViewState } from './types';

export const initialAgentRunState: AgentRunViewState = {
  highest_sequence: 0,
  status: 'idle',
  answer: '',
  router: null,
  plan: null,
  rounds: [],
  tools: [],
  citations: [],
  usage: null,
  error: null,
  resumable: false,
};

const stringValue = (value: unknown, fallback = '') => typeof value === 'string' ? value : fallback;
const numberValue = (value: unknown, fallback = 0) => typeof value === 'number' ? value : fallback;

export type ResetAction = { type: 'RESET_AGENT_RUN_STATE' };

export function reduceAgentEvent(state: AgentRunViewState, event: AgentEvent | ResetAction): AgentRunViewState {
  if (event.type === 'RESET_AGENT_RUN_STATE') {
    return { ...initialAgentRunState };
  }
  if (event.sequence <= state.highest_sequence) return state;
  const next: AgentRunViewState = { ...state, highest_sequence: event.sequence };
  const payload = event.payload;

  switch (event.type) {
    case 'agent.run.queued':
      return { ...next, status: 'queued', resumable: false };
    case 'agent.run.started':
      return { ...next, status: 'running', resumable: false, error: null, answer: '' };
    case 'agent.run.completed':
      return { ...next, status: 'completed', resumable: false };
    case 'agent.run.cancelled':
      return { ...next, status: 'cancelled', resumable: true };
    case 'agent.run.failed':
      return {
        ...next,
        status: 'failed',
        resumable: true,
        error: {
          code: stringValue(payload.error_code, 'RUN_FAILED'),
          message: stringValue(payload.error_message, 'Agent 运行失败'),
        },
      };
    case 'agent.router.completed':
      return {
        ...next,
        router: {
          execution_mode: payload.execution_mode === 'plan_execute' ? 'plan_execute' : 'react',
          reason_summary: stringValue(payload.reason_summary),
        },
      };
    case 'agent.step.started':
    case 'agent.step.completed': {
      if (!state.plan) return next;
      const status = event.type === 'agent.step.started' ? 'running' : 'completed';
      const stepNo = numberValue(payload.step_no);
      return {
        ...next,
        plan: {
          ...state.plan,
          steps: state.plan.steps.map((step) => step.step_no === stepNo
            ? { ...step, status }
            : step),
        },
      };
    }
    case 'agent.react.round.started':
      return {
        ...next,
        rounds: [...state.rounds, {
          round_no: numberValue(payload.round_no),
          status: 'running',
          action_summary: stringValue(payload.action_summary),
          ...(payload.tool_name ? { tool_name: stringValue(payload.tool_name) } : {}),
        }],
      };
    case 'agent.tool.started':
      return {
        ...next,
        tools: [...state.tools, {
          tool_call_id: stringValue(payload.tool_call_id),
          tool_name: stringValue(payload.tool_name),
          status: 'running',
          input_summary: stringValue(payload.input_summary),
        }],
      };
    case 'agent.tool.completed':
    case 'agent.tool.call.failed': {
      const id = stringValue(payload.tool_call_id);
      return {
        ...next,
        tools: state.tools.map((tool) => tool.tool_call_id === id
          ? { ...tool, status: event.type === 'agent.tool.completed' ? 'succeeded' : 'failed', output_summary: stringValue(payload.output_summary ?? payload.error_message) }
          : tool),
      };
    }
    case 'agent.answer.delta':
      return { ...next, answer: state.answer + stringValue(payload.delta) };
    case 'citation.created': {
      const { reasoning: _reasoning, ...visibleCitation } = payload;
      void _reasoning;
      return { ...next, citations: [...state.citations, visibleCitation] };
    }
    case 'usage.updated':
      return {
        ...next,
        usage: {
          input_tokens: numberValue(payload.input_tokens),
          output_tokens: numberValue(payload.output_tokens),
          total_tokens: numberValue(payload.total_tokens),
        },
      };
    default:
      return next;
  }
}
