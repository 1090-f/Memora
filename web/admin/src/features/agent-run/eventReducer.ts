import type { AgentEvent, AgentRun, AgentRunViewState } from './types';

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
export type QueueAction = { type: 'SET_AGENT_RUN_QUEUED' };
export type CancelledAction = { type: 'SET_AGENT_RUN_CANCELLED' };
export type HydrateAction = { type: 'HYDRATE_AGENT_RUN_STATE'; run: AgentRun };
export type AgentRunAction = AgentEvent | ResetAction | QueueAction | CancelledAction | HydrateAction;

export function reduceAgentEvent(state: AgentRunViewState, event: AgentRunAction): AgentRunViewState {
  if (event.type === 'RESET_AGENT_RUN_STATE') {
    return { ...initialAgentRunState };
  }
  if (event.type === 'SET_AGENT_RUN_QUEUED') {
    return { ...initialAgentRunState, status: 'queued' };
  }
  if (event.type === 'SET_AGENT_RUN_CANCELLED') {
    return { ...initialAgentRunState, status: 'cancelled', resumable: true };
  }
  if (event.type === 'HYDRATE_AGENT_RUN_STATE') {
    const run = event.run;
    const hasUsage = (run.input_tokens ?? 0) > 0 || (run.output_tokens ?? 0) > 0 || (run.total_tokens ?? 0) > 0;
    return {
      ...initialAgentRunState,
      status: run.status,
      answer: run.final_result ?? '',
      router: run.execution_mode ? {
        execution_mode: run.execution_mode,
        reason_summary: run.router_reason_summary ?? run.router_reason ?? '',
      } : null,
      usage: hasUsage ? {
        input_tokens: run.input_tokens ?? 0,
        output_tokens: run.output_tokens ?? 0,
        total_tokens: run.total_tokens ?? 0,
      } : null,
      error: run.error_code || run.error_message ? {
        code: run.error_code ?? 'RUN_FAILED',
        message: run.error_message ?? 'Agent 运行失败',
      } : null,
      resumable: run.status === 'failed' || run.status === 'cancelled',
    };
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
      return {
        ...next,
        status: 'completed',
        resumable: false,
        // Plan-Execute 模式没有 agent.answer.delta 事件，需从 final_result 填充 answer
        answer: next.answer || stringValue(payload.final_result),
      };
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
    case 'agent.plan.created':
      // 初始化计划展示面板，包含版本号、目标、步骤列表等完整信息。
      return {
        ...next,
        plan: {
          version: numberValue(payload.version, 1),
          reason_summary: stringValue(payload.goal),
          steps: Array.isArray(payload.steps)
            ? payload.steps.map((step: Record<string, unknown>) => ({
                step_no: numberValue(step.step_no),
                title: stringValue(step.title),
                status: stringValue(step.status, 'pending'),
              }))
            : [],
        },
      };
    case 'agent.plan.replanned':
      // 计划重新规划后更新计划面板，版本号递增、步骤列表刷新。
      if (!state.plan) return next;
      return {
        ...next,
        plan: {
          version: numberValue(payload.version, state.plan.version),
          reason_summary: stringValue(payload.goal) || state.plan.reason_summary,
          steps: Array.isArray(payload.steps)
            ? payload.steps.map((step: Record<string, unknown>) => ({
                step_no: numberValue(step.step_no),
                title: stringValue(step.title),
                status: stringValue(step.status, 'pending'),
              }))
            : state.plan.steps,
        },
      };
    case 'agent.step.started':
    case 'agent.step.completed':
    case 'agent.plan.step.started':
    case 'agent.plan.step.completed': {
      if (!state.plan) return next;
      const stepStatus = event.type === 'agent.step.started' || event.type === 'agent.plan.step.started' ? 'running' : 'completed';
      const stepNo = numberValue(payload.step_no);
      return {
        ...next,
        plan: {
          ...state.plan,
          steps: state.plan.steps.map((step) => step.step_no === stepNo
            ? { ...step, status: stepStatus }
            : step),
        },
      };
    }
    case 'agent.react.round.started':
      {
        const roundNo = numberValue(payload.round_no, numberValue(payload.round));
        return {
          ...next,
          rounds: [...state.rounds, {
            round_no: roundNo,
            status: 'running',
            action_summary: stringValue(payload.action_summary),
            ...(payload.tool_name ? { tool_name: stringValue(payload.tool_name) } : {}),
          }],
        };
      }
    case 'agent.react.round.completed': {
      const roundNo = numberValue(payload.round_no, numberValue(payload.round));
      return {
        ...next,
        rounds: state.rounds.map((round) => round.round_no === roundNo ? { ...round, status: 'completed' } : round),
      };
    }
    case 'agent.tool.started':
      return {
        ...next,
        tools: [...state.tools, {
          tool_call_id: stringValue(payload.tool_call_id) || stringValue(payload.call_id),
          tool_name: stringValue(payload.tool_name),
          status: 'running',
          input_summary: stringValue(payload.input_summary),
        }],
      };
    case 'agent.tool.completed':
    case 'agent.tool.call.failed': {
      const id = stringValue(payload.tool_call_id) || stringValue(payload.call_id);
      return {
        ...next,
        tools: state.tools.map((tool) => tool.tool_call_id === id
          ? { ...tool, status: event.type === 'agent.tool.completed' ? 'completed' : 'failed', output_summary: stringValue(payload.output_summary ?? payload.summary ?? payload.error_message) }
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
