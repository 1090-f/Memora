import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { SpanExplorer } from './SpanExplorer';
import type { TraceSpan } from '../types';

const spans: TraceSpan[] = [
  { trace_id: 'trace', span_id: 'root', name: 'POST /api/v1/agent/runs', kind: 'server', status_code: 'Ok', started_at: '2026-09-04T00:00:00Z', ended_at: '2026-09-04T00:00:01Z', duration_ms: 1000, attributes: { 'http.request.method': 'POST' }, events: [], service_name: 'memora-api' },
  { trace_id: 'trace', span_id: 'child', parent_span_id: 'root', name: 'agent.run', kind: 'consumer', status_code: 'Error', status_message: 'failed', started_at: '2026-09-04T00:00:00.1Z', ended_at: '2026-09-04T00:00:00.9Z', duration_ms: 800, attributes: { 'memora.run_id': 'run-1' }, events: [] },
];

describe('SpanExplorer', () => {
  it('renders waterfall rows and selects span details', () => {
    render(<SpanExplorer spans={spans} />);
    expect(screen.getByText('Trace Explorer')).toBeInTheDocument();
    fireEvent.click(screen.getByText('agent.run'));
    expect(screen.getByText('Span ID：child')).toBeInTheDocument();
    expect(screen.getByText(/状态：Error/)).toBeInTheDocument();
  });

  it('explains empty historical traces', () => {
    render(<SpanExplorer spans={[]} />);
    expect(screen.getByText(/旧运行或服务升级前/)).toBeInTheDocument();
  });
});
