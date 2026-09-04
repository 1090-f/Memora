import type { TraceSpan } from './types';

export interface TraceSpanRow {
  span: TraceSpan;
  depth: number;
  offsetPercent: number;
  widthPercent: number;
  orphaned: boolean;
}

export function buildTraceSpanRows(spans: TraceSpan[]): TraceSpanRow[] {
  if (spans.length === 0) return [];
  const sorted = [...spans].sort((left, right) => Date.parse(left.started_at) - Date.parse(right.started_at));
  const byId = new Map(sorted.map((span) => [span.span_id, span]));
  const children = new Map<string, TraceSpan[]>();
  for (const span of sorted) {
    const parent = span.parent_span_id;
    if (parent && byId.has(parent)) {
      const values = children.get(parent) ?? [];
      values.push(span);
      children.set(parent, values);
    }
  }
  const traceStart = Math.min(...sorted.map((span) => Date.parse(span.started_at)));
  const traceEnd = Math.max(...sorted.map((span) => Date.parse(span.ended_at)));
  const traceDuration = Math.max(1, traceEnd - traceStart);
  const rows: TraceSpanRow[] = [];
  const visited = new Set<string>();
  const append = (span: TraceSpan, depth: number, orphaned: boolean) => {
    if (visited.has(span.span_id)) return;
    visited.add(span.span_id);
    const startedAt = Date.parse(span.started_at);
    const endedAt = Date.parse(span.ended_at);
    rows.push({
      span,
      depth,
      offsetPercent: Math.max(0, Math.min(100, ((startedAt - traceStart) / traceDuration) * 100)),
      widthPercent: Math.max(0.4, Math.min(100, ((Math.max(startedAt, endedAt) - startedAt) / traceDuration) * 100)),
      orphaned,
    });
    for (const child of children.get(span.span_id) ?? []) append(child, depth + 1, false);
  };
  for (const span of sorted) {
    if (!span.parent_span_id || !byId.has(span.parent_span_id)) append(span, 0, Boolean(span.parent_span_id));
  }
  for (const span of sorted) append(span, 0, true);
  return rows;
}

export function traceDurationMS(spans: TraceSpan[]) {
  if (spans.length === 0) return 0;
  const start = Math.min(...spans.map((span) => Date.parse(span.started_at)));
  const end = Math.max(...spans.map((span) => Date.parse(span.ended_at)));
  return Math.max(0, end - start);
}
