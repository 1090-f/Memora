import { describe, expect, it } from 'vitest';
import { initialAgentRunState, reduceAgentEvent } from './eventReducer';
import type { AgentEvent } from './types';

const event = (sequence: number, type: AgentEvent['type'], payload: Record<string, unknown> = {}): AgentEvent => ({
  run_id: 'run-1', sequence, timestamp: '2026-08-04T10:00:00Z', type, payload,
});

describe('reduceAgentEvent', () => {
  it('reduces a visible ReAct sequence without retaining hidden reasoning', () => {
    const events: AgentEvent[] = [
      event(1, 'run.started'),
      event(2, 'router.selected', { execution_mode: 'react', reason_summary: '动态检索', reasoning: 'secret chain' }),
      event(3, 'agent.round.started', { round_no: 1, action_summary: '检索知识库', reasoning: 'hidden' }),
      event(4, 'tool.call.started', { tool_call_id: 'tool-1', tool_name: 'knowledge_search', input_summary: '检索 ReAct' }),
      event(5, 'tool.call.completed', { tool_call_id: 'tool-1', output_summary: '返回 6 个片段' }),
      event(6, 'answer.delta', { delta: 'ReAct 与 RAG 可以结合。' }),
      event(7, 'citation.created', { document_id: 'doc-1', document_title: 'Eino.md', quoted_text: '摘要' }),
      event(8, 'usage.updated', { input_tokens: 100, output_tokens: 20, total_tokens: 120 }),
      event(9, 'run.completed'),
    ];

    const state = events.reduce(reduceAgentEvent, initialAgentRunState);

    expect(state.status).toBe('completed');
    expect(state.answer).toBe('ReAct 与 RAG 可以结合。');
    expect(state.router).toEqual({ execution_mode: 'react', reason_summary: '动态检索' });
    expect(state.rounds[0]).toMatchObject({ round_no: 1, action_summary: '检索知识库' });
    expect(state.tools[0]).toMatchObject({ status: 'succeeded', output_summary: '返回 6 个片段' });
    expect(JSON.stringify(state)).not.toContain('secret chain');
    expect(JSON.stringify(state)).not.toContain('hidden');
  });

  it('tracks plan steps, replans, failures and cancellation while preserving answer text', () => {
    const events: AgentEvent[] = [
      event(1, 'plan.created', { version: 1, steps: [{ step_no: 1, title: '检索资料', status: 'pending' }] }),
      event(2, 'step.started', { step_no: 1 }),
      event(3, 'answer.delta', { delta: '已找到部分内容' }),
      event(4, 'step.failed', { step_no: 1, error_message: '上游超时' }),
      event(5, 'plan.replanned', { version: 2, reason_summary: '切换资料源' }),
      event(6, 'run.cancelled'),
    ];
    const state = events.reduce(reduceAgentEvent, initialAgentRunState);

    expect(state.answer).toBe('已找到部分内容');
    expect(state.status).toBe('cancelled');
    expect(state.resumable).toBe(true);
    expect(state.plan?.version).toBe(2);
    expect(state.plan?.steps[0].status).toBe('failed');
  });

  it('ignores duplicate and out-of-order sequences', () => {
    const state = [
      event(1, 'answer.delta', { delta: 'A' }),
      event(3, 'answer.delta', { delta: 'C' }),
      event(3, 'answer.delta', { delta: 'duplicate' }),
      event(2, 'answer.delta', { delta: 'late' }),
    ].reduce(reduceAgentEvent, initialAgentRunState);

    expect(state.answer).toBe('AC');
    expect(state.highest_sequence).toBe(3);
  });

  it('exposes a safe run failure and keeps prior answer output', () => {
    const state = [
      event(1, 'answer.delta', { delta: '可保留的回答' }),
      event(2, 'run.failed', { error_code: 'MODEL_CALL_FAILED', error_message: '模型暂时不可用', reasoning: 'do not keep' }),
    ].reduce(reduceAgentEvent, initialAgentRunState);

    expect(state.status).toBe('failed');
    expect(state.answer).toBe('可保留的回答');
    expect(state.error).toEqual({ code: 'MODEL_CALL_FAILED', message: '模型暂时不可用' });
    expect(JSON.stringify(state)).not.toContain('do not keep');
  });
});
