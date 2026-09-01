DROP TABLE IF EXISTS document_processing_events;
DROP INDEX IF EXISTS idx_agent_events_timestamp;
DROP INDEX IF EXISTS idx_agent_events_trace_id;
DROP INDEX IF EXISTS idx_agent_runs_trace_id;
DROP INDEX IF EXISTS idx_import_tasks_trace_id;
ALTER TABLE agent_events DROP CONSTRAINT IF EXISTS chk_agent_events_stage_status;
ALTER TABLE agent_events
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS stage,
    DROP COLUMN IF EXISTS request_id,
    DROP COLUMN IF EXISTS trace_id;
ALTER TABLE agent_runs
    DROP COLUMN IF EXISTS request_id,
    DROP COLUMN IF EXISTS trace_id;
ALTER TABLE import_tasks
    DROP COLUMN IF EXISTS error_code,
    DROP COLUMN IF EXISTS request_id,
    DROP COLUMN IF EXISTS trace_id;
