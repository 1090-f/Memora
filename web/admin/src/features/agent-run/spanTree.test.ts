import { describe, expect, it } from 'vitest';
import { buildTraceSpanRows, traceDurationMS } from './spanTree';
import type { TraceSpan } from './types';

const span = (overrides: Partial<TraceSpan>): TraceSpan => ({
  trace_id: 'trace', span_id: 'root', name: 'root', kind: 'server', status_code: 'Ok',
  started_at: '2026-09-04T00:00:00.000Z', ended_at: '2026-09-04T00:00:01.000Z',
  duration_ms: 1000, attributes: {}, events: [], ...overrides,
});

describe('buildTraceSpanRows', () => {
  it('builds parent-child order and waterfall positions', () => {
    const rows = buildTraceSpanRows([
      span({}),
      span({ span_id: 'child', parent_span_id: 'root', name: 'agent.run', started_at: '2026-09-04T00:00:00.250Z', ended_at: '2026-09-04T00:00:00.750Z', duration_ms: 500 }),
    ]);
    expect(rows.map((row) => [row.span.span_id, row.depth])).toEqual([['root', 0], ['child', 1]]);
    expect(rows[1].offsetPercent).toBe(25);
    expect(rows[1].widthPercent).toBe(50);
    expect(traceDurationMS(rows.map((row) => row.span))).toBe(1000);
  });

  it('keeps an orphan span visible as a root', () => {
    const rows = buildTraceSpanRows([span({ span_id: 'orphan', parent_span_id: 'missing' })]);
    expect(rows[0]).toMatchObject({ depth: 0, orphaned: true });
  });
});
