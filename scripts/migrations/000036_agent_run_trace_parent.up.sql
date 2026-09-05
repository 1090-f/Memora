ALTER TABLE agent_runs
    ADD COLUMN trace_parent_span_id varchar(16),
    ADD COLUMN trace_sampled boolean NOT NULL DEFAULT true;
