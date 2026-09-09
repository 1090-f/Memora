ALTER TABLE agent_runs
    DROP COLUMN IF EXISTS trace_parent_span_id,
    DROP COLUMN IF EXISTS trace_sampled;
