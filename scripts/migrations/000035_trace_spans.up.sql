CREATE TABLE trace_spans (
    trace_id varchar(32) NOT NULL,
    span_id varchar(16) NOT NULL,
    parent_span_id varchar(16),
    name varchar(255) NOT NULL,
    kind varchar(24) NOT NULL,
    status_code varchar(16) NOT NULL,
    status_message varchar(500),
    started_at timestamptz NOT NULL,
    ended_at timestamptz NOT NULL,
    duration_ms bigint NOT NULL CHECK (duration_ms >= 0),
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
    events jsonb NOT NULL DEFAULT '[]'::jsonb,
    service_name varchar(128),
    instrumentation_scope varchar(255),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (trace_id, span_id)
);

CREATE INDEX idx_trace_spans_trace_started_at
    ON trace_spans(trace_id, started_at ASC);
CREATE INDEX idx_trace_spans_created_at
    ON trace_spans(created_at);
