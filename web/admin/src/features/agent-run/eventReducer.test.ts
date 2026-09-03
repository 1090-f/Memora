import { describe, expect, it } from 'vitest';

import { initialAgentRunState, reduceAgentEvent } from './eventReducer';
import type { AgentEvent, AgentEventType, AgentRunViewState } from './types';

const event = (sequence: number, type: AgentEventType, payload: Record<string, unknown> = {}): AgentEvent => ({
  run_id: 'run-1',
  sequence,
  timestamp: '2026-09-03T00:00:00Z',
  type,
  payload,
});

describe('reduceAgentEvent', () => {
  it('ignores duplicate and out-of-order sequences', () => {
    const started = reduceAgentEvent(initialAgentRunState, event(2, 'agent.run.started'));
    expect(started.status).toBe('running');
    expect(reduceAgentEvent(started, event(2, 'agent.run.failed'))).toBe(started);
    expect(reduceAgentEvent(started, event(1, 'agent.run.failed'))).toBe(started);
  });

  it('merges consecutive answer deltas and keeps the first sequence', () => {
    let state = reduceAgentEvent(initialAgentRunState, event(1, 'agent.answer.delta', { delta: '你好' }));
    state = reduceAgentEvent(state, event(2, 'agent.answer.delta', { delta: '，世界' }));

    expect(state.answer).toBe('你好，世界');
    expect(state.timeline).toEqual([{ kind: 'answer', sequence: 1, delta: '你好，世界' }]);
  });

  it('merges a completed stage into its running timeline entry', () => {
    let state = reduceAgentEvent(initialAgentRunState, event(1, 'agent.stage.updated', {
      stage: 'context_build', status: 'running', started_at: '2026-09-03T00:00:00Z',
    }));
    state = reduceAgentEvent(state, event(2, 'agent.stage.updated', {
      stage: 'context_build', status: 'succeeded', duration_ms: 18, summary: '上下文构造完成',
    }));

    expect(state.timeline).toHaveLength(1);
    expect(state.timeline[0]).toMatchObject({
      kind: 'stage', sequence: 1, stage: 'context_build', status: 'succeeded', duration_ms: 18,
    });
  });

  it('merges tool failure details and marks the run as resumable on failure', () => {
    let state = reduceAgentEvent(initialAgentRunState, event(1, 'agent.tool.started', {
      tool_call_id: 'call-1', tool_name: 'mcp.search',
    }));
    state = reduceAgentEvent(state, event(2, 'agent.tool.call.failed', {
      tool_call_id: 'call-1', tool_name: 'mcp.search', error_message: '连接失败',
    }));
    state = reduceAgentEvent(state, event(3, 'agent.run.failed', {
      error_code: 'MODEL_CALL_FAILED', error_message: '模型暂不可用',
    }));

    expect(state.tools[0]).toMatchObject({ status: 'failed', output_summary: '连接失败' });
    expect(state.timeline[0]).toMatchObject({ kind: 'tool', status: 'failed', error_message: '连接失败' });
    expect(state).toMatchObject<Partial<AgentRunViewState>>({
      status: 'failed', resumable: true, error: { code: 'MODEL_CALL_FAILED', message: '模型暂不可用' },
    });
  });

  it('uses the persisted final result when no answer delta exists', () => {
    const state = reduceAgentEvent(initialAgentRunState, event(1, 'agent.run.completed', {
      final_result: '计划模式最终回答',
    }));
    expect(state.status).toBe('completed');
    expect(state.answer).toBe('计划模式最终回答');
  });
});
