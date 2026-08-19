import type { AgentEvent, AgentRun, AgentRunViewState, AgentTimelineEntry } from './types';

export const initialAgentRunState: AgentRunViewState = {
  highest_sequence: 0,
  status: 'idle',
  timeline: [],
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

// 追加一条执行链路记录。
function withEntry(state: AgentRunViewState, entry: AgentTimelineEntry): AgentRunViewState {
  return { ...state, timeline: [...state.timeline, entry] };
}

// 从后往前查找最后一条满足条件的记录并原地更新，返回新数组。
function updateLastEntry(
  timeline: AgentTimelineEntry[],
  predicate: (entry: AgentTimelineEntry) => boolean,
  updater: (entry: AgentTimelineEntry) => AgentTimelineEntry,
): AgentTimelineEntry[] {
  for (let i = timeline.length - 1; i >= 0; i--) {
    if (predicate(timeline[i])) {
      const next = [...timeline];
      next[i] = updater(timeline[i]);
      return next;
    }
  }
  return timeline;
}

// 追加或累计 answer 增量，保证 answer 只占一个时间点。
function withAnswerDelta(state: AgentRunViewState, sequence: number, delta: string): AgentRunViewState {
  const last = state.timeline[state.timeline.length - 1];
  if (last && last.kind === 'answer') {
    const next = [...state.timeline];
    next[next.length - 1] = { ...last, delta: last.delta + delta };
    return { ...state, timeline: next };
  }
  return withEntry(state, { kind: 'answer', sequence, delta });
}

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
      return withEntry({ ...next, status: 'queued', resumable: false }, { kind: 'status', sequence: event.sequence, title: '排队中', status: 'queued' });
    case 'agent.run.started':
      return withEntry({ ...next, status: 'running', resumable: false, error: null, answer: '' }, { kind: 'status', sequence: event.sequence, title: '开始执行', status: 'running' });
    case 'agent.run.completed':
      return withEntry({
        ...next,
        status: 'completed',
        resumable: false,
        // Plan-Execute 模式没有 agent.answer.delta 事件，需从 final_result 填充 answer
        answer: next.answer || stringValue(payload.final_result),
      }, { kind: 'status', sequence: event.sequence, title: '运行完成', status: 'completed' });
    case 'agent.run.cancelled':
      return withEntry({ ...next, status: 'cancelled', resumable: true }, { kind: 'status', sequence: event.sequence, title: '运行已取消', status: 'cancelled' });
    case 'agent.run.failed':
      return withEntry({
        ...next,
        status: 'failed',
        resumable: true,
        error: {
          code: stringValue(payload.error_code, 'RUN_FAILED'),
          message: stringValue(payload.error_message, 'Agent 运行失败'),
        },
      }, {
        kind: 'status',
        sequence: event.sequence,
        title: '运行失败',
        status: 'failed',
        error_message: stringValue(payload.error_message),
      });
    case 'agent.router.completed':
      return withEntry({
        ...next,
        router: {
          execution_mode: payload.execution_mode === 'plan_execute' ? 'plan_execute' : 'react',
          reason_summary: stringValue(payload.reason_summary),
        },
      }, {
        kind: 'router',
        sequence: event.sequence,
        execution_mode: payload.execution_mode === 'plan_execute' ? 'plan_execute' : 'react',
        reason_summary: stringValue(payload.reason_summary),
      });
    case 'agent.plan.created':
      // 初始化计划展示面板，包含版本号、目标、步骤列表等完整信息。
      return withEntry({
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
      }, {
        kind: 'plan_created',
        sequence: event.sequence,
        version: numberValue(payload.version, 1),
        goal: stringValue(payload.goal),
        step_count: Array.isArray(payload.steps) ? payload.steps.length : 0,
        replanned: false,
      });
    case 'agent.plan.replanned':
      // 计划重新规划后更新计划面板，版本号递增、步骤列表刷新。
      if (!state.plan) return next;
      return withEntry({
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
      }, {
        kind: 'plan_created',
        sequence: event.sequence,
        version: numberValue(payload.version, state.plan.version),
        goal: stringValue(payload.goal),
        step_count: Array.isArray(payload.steps) ? payload.steps.length : (state.plan.steps.length || 0),
        replanned: true,
      });
    case 'agent.step.started':
    case 'agent.step.completed':
    case 'agent.plan.step.started':
    case 'agent.plan.step.completed': {
      if (!state.plan) return next;
      const started = event.type === 'agent.step.started' || event.type === 'agent.plan.step.started';
      const stepStatus = started ? 'running' : payload.success === false ? 'failed' : 'completed';
      const stepNo = numberValue(payload.step_no);
      const version = state.plan.version;
      const title = stringValue(payload.title, `步骤 ${stepNo}`);
      const nextPlan = {
        ...next,
        plan: {
          ...state.plan,
          steps: state.plan.steps.map((step) => step.step_no === stepNo
            ? { ...step, status: stepStatus }
            : step),
        },
      };
      if (started) {
        return withEntry(nextPlan, { kind: 'plan_step', sequence: event.sequence, version, step_no: stepNo, title, status: 'running' });
      }
      const timeline = updateLastEntry(
        nextPlan.timeline,
        (entry): entry is Extract<AgentTimelineEntry, { kind: 'plan_step' }> => entry.kind === 'plan_step' && entry.version === version && entry.step_no === stepNo,
        (entry) => ({ ...entry, status: stepStatus }),
      );
      return { ...nextPlan, timeline };
    }
    case 'agent.react.round.started':
      {
        const roundNo = numberValue(payload.round_no, numberValue(payload.round));
        return withEntry({
          ...next,
          rounds: [...state.rounds, {
            round_no: roundNo,
            status: 'running',
            action_summary: stringValue(payload.action_summary),
            ...(payload.tool_name ? { tool_name: stringValue(payload.tool_name) } : {}),
          }],
        }, {
          kind: 'round',
          sequence: event.sequence,
          round_no: roundNo,
          status: 'running',
          action_summary: stringValue(payload.action_summary),
        });
      }
    case 'agent.react.round.completed': {
      const roundNo = numberValue(payload.round_no, numberValue(payload.round));
      const nextState = {
        ...next,
        rounds: state.rounds.map((round) => round.round_no === roundNo ? { ...round, status: 'completed' } : round),
      };
      const timeline = updateLastEntry(
        nextState.timeline,
        (entry): entry is Extract<AgentTimelineEntry, { kind: 'round' }> => entry.kind === 'round' && entry.round_no === roundNo,
        (entry) => ({ ...entry, status: 'completed' }),
      );
      return { ...nextState, timeline };
    }
    case 'agent.tool.started':
      return withEntry({
        ...next,
        tools: [...state.tools, {
          tool_call_id: stringValue(payload.tool_call_id) || stringValue(payload.call_id),
          tool_name: stringValue(payload.tool_name),
          status: 'running',
          input_summary: stringValue(payload.input_summary),
        }],
      }, {
        kind: 'tool',
        sequence: event.sequence,
        tool_call_id: stringValue(payload.tool_call_id) || stringValue(payload.call_id),
        tool_name: stringValue(payload.tool_name),
        status: 'running',
        input_summary: stringValue(payload.input_summary),
      });
    case 'agent.tool.completed':
    case 'agent.tool.call.failed': {
      const id = stringValue(payload.tool_call_id) || stringValue(payload.call_id);
      const succeeded = event.type === 'agent.tool.completed';
      const toolStatus = succeeded ? 'completed' : 'failed';
      const outputSummary = stringValue(payload.output_summary ?? payload.summary ?? payload.error_message);
      const nextState = {
        ...next,
        tools: state.tools.map((tool) => tool.tool_call_id === id
          ? { ...tool, status: toolStatus, output_summary: outputSummary }
          : tool),
      };
      const timeline = updateLastEntry(
        nextState.timeline,
        (entry): entry is Extract<AgentTimelineEntry, { kind: 'tool' }> => entry.kind === 'tool' && entry.tool_call_id === id,
        (entry) => ({
          ...entry,
          status: toolStatus,
          ...(outputSummary ? { output_summary: outputSummary } : {}),
          ...(!succeeded ? { error_message: stringValue(payload.error_message) } : {}),
        }),
      );
      return { ...nextState, timeline };
    }
    case 'agent.answer.delta':
      return { ...withAnswerDelta(next, event.sequence, stringValue(payload.delta)), answer: state.answer + stringValue(payload.delta) };
    case 'citation.created': {
      const { reasoning: _reasoning, ...visibleCitation } = payload;
      void _reasoning;
      return withEntry(
        { ...next, citations: [...state.citations, visibleCitation] },
        { kind: 'citation', sequence: event.sequence, citation: visibleCitation },
      );
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
