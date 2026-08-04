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

export function reduceAgentEvent(state: AgentRunViewState, event: AgentEvent): AgentRunViewState {
  if (event.sequence <= state.highest_sequence) return state;
  const next: AgentRunViewState = { ...state, highest_sequence: event.sequence };
  const payload = event.payload;

  switch (event.type) {
    case 'run.started':
      return { ...next, status: 'running', resumable: false, error: null };
    case 'run.completed':
      return { ...next, status: 'completed', resumable: false };
    case 'run.cancelled':
      return { ...next, status: 'cancelled', resumable: true };
    case 'run.failed':
      return {
        ...next,
        status: 'failed',
        resumable: true,
        error: {
          code: stringValue(payload.error_code, 'RUN_FAILED'),
          message: stringValue(payload.error_message, 'Agent 运行失败'),
        },
      };
    case 'router.selected':
      return {
        ...next,
        router: {
          execution_mode: payload.execution_mode === 'plan_execute' ? 'plan_execute' : 'react',
          reason_summary: stringValue(payload.reason_summary),
        },
      };
    case 'plan.created': {
      const rawSteps = Array.isArray(payload.steps) ? payload.steps : [];
      return {
        ...next,
        plan: {
          version: numberValue(payload.version, 1),
          steps: rawSteps.map((item) => {
            const step = item as Record<string, unknown>;
            return {
              step_no: numberValue(step.step_no),
              title: stringValue(step.title),
              status: stringValue(step.status, 'pending'),
            };
          }),
        },
      };
    }
    case 'plan.replanned':
      return {
        ...next,
        plan: {
          version: numberValue(payload.version, (state.plan?.version ?? 0) + 1),
          reason_summary: stringValue(payload.reason_summary),
          steps: state.plan?.steps ?? [],
        },
      };
    case 'step.started':
    case 'step.completed':
    case 'step.failed': {
      if (!state.plan) return next;
      const status = event.type === 'step.started' ? 'running' : event.type === 'step.completed' ? 'completed' : 'failed';
      const stepNo = numberValue(payload.step_no);
      return {
        ...next,
        plan: {
          ...state.plan,
          steps: state.plan.steps.map((step) => step.step_no === stepNo
            ? { ...step, status, ...(status === 'failed' ? { error_message: stringValue(payload.error_message) } : {}) }
            : step),
        },
      };
    }
    case 'agent.round.started':
      return {
        ...next,
        rounds: [...state.rounds, {
          round_no: numberValue(payload.round_no),
          status: 'running',
          action_summary: stringValue(payload.action_summary),
          ...(payload.tool_name ? { tool_name: stringValue(payload.tool_name) } : {}),
        }],
      };
    case 'tool.call.started':
      return {
        ...next,
        tools: [...state.tools, {
          tool_call_id: stringValue(payload.tool_call_id),
          tool_name: stringValue(payload.tool_name),
          status: 'running',
          input_summary: stringValue(payload.input_summary),
        }],
      };
    case 'tool.call.completed':
    case 'tool.call.failed': {
      const id = stringValue(payload.tool_call_id);
      return {
        ...next,
        tools: state.tools.map((tool) => tool.tool_call_id === id
          ? { ...tool, status: event.type === 'tool.call.completed' ? 'succeeded' : 'failed', output_summary: stringValue(payload.output_summary ?? payload.error_message) }
          : tool),
      };
    }
    case 'answer.delta':
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
