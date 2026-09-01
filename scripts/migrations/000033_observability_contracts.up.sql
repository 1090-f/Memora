ALTER TABLE agent_runs
    ADD COLUMN trace_id varchar(32),
    ADD COLUMN request_id varchar(128);

ALTER TABLE import_tasks
    ADD COLUMN trace_id varchar(32),
    ADD COLUMN request_id varchar(128),
    ADD COLUMN error_code varchar(64);

ALTER TABLE agent_events
    ADD COLUMN trace_id varchar(32),
    ADD COLUMN request_id varchar(128),
    ADD COLUMN stage varchar(40),
    ADD COLUMN status varchar(20),
    ADD CONSTRAINT chk_agent_events_stage_status
        CHECK (status IS NULL OR status IN ('pending', 'running', 'succeeded', 'failed', 'skipped'));

CREATE INDEX idx_agent_runs_trace_id ON agent_runs(trace_id) WHERE trace_id IS NOT NULL;
CREATE INDEX idx_agent_events_trace_id ON agent_events(trace_id) WHERE trace_id IS NOT NULL;
CREATE INDEX idx_agent_events_timestamp ON agent_events(timestamp);
CREATE INDEX idx_import_tasks_trace_id ON import_tasks(trace_id) WHERE trace_id IS NOT NULL;

CREATE TABLE document_processing_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    document_id uuid NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    task_id uuid REFERENCES import_tasks(id) ON DELETE SET NULL,
    stage varchar(40) NOT NULL CHECK (stage IN ('upload','parse','normalize','chunk','embed','index','preview')),
    status varchar(20) NOT NULL CHECK (status IN ('pending','running','succeeded','failed','skipped')),
    started_at timestamptz,
    ended_at timestamptz,
    duration_ms bigint CHECK (duration_ms IS NULL OR duration_ms >= 0),
    attempt integer NOT NULL DEFAULT 1 CHECK (attempt > 0),
    error_code varchar(64),
    error_message text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    trace_id varchar(32),
    request_id varchar(128),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_document_processing_events_document_created ON document_processing_events(document_id, created_at DESC);
CREATE INDEX idx_document_processing_events_task_stage ON document_processing_events(task_id, stage, created_at) WHERE task_id IS NOT NULL;
CREATE INDEX idx_document_processing_events_created_at ON document_processing_events(created_at);
