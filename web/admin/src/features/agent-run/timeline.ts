import type { AgentRunViewState, AgentTimelineEntry } from './types';

// buildFallbackTimeline 在事件回放缺失（timeline 为空）时，从 reducer 其余状态兜底生成执行链路。
export function buildFallbackTimeline(state: AgentRunViewState): AgentTimelineEntry[] {
  const entries: AgentTimelineEntry[] = [];
  let sequence = 1;
  if (state.router) {
    entries.push({
      kind: 'router',
      sequence: sequence++,
      execution_mode: state.router.execution_mode,
      reason_summary: state.router.reason_summary,
    });
  }
  if (state.plan) {
    entries.push({
      kind: 'plan_created',
      sequence: sequence++,
      version: state.plan.version,
      goal: state.plan.reason_summary ?? '',
      step_count: state.plan.steps.length,
      replanned: false,
    });
    for (const step of state.plan.steps) {
      entries.push({
        kind: 'plan_step',
        sequence: sequence++,
        version: state.plan.version,
        step_no: step.step_no,
        title: step.title,
        status: step.status,
      });
    }
  } else {
    for (const round of state.rounds) {
      entries.push({
        kind: 'round',
        sequence: sequence++,
        round_no: round.round_no,
        status: round.status,
        action_summary: round.action_summary,
      });
    }
  }
  for (const tool of state.tools) {
    entries.push({
      kind: 'tool',
      sequence: sequence++,
      tool_call_id: tool.tool_call_id,
      tool_name: tool.tool_name,
      status: tool.status,
      input_summary: tool.input_summary,
      output_summary: tool.output_summary,
    });
  }
  return entries;
}

// timelineEntries 优先返回真实事件链路，缺失时使用兜底生成。
export function timelineEntries(state: AgentRunViewState): AgentTimelineEntry[] {
  return state.timeline.length > 0 ? state.timeline : buildFallbackTimeline(state);
}