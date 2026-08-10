ALTER TABLE import_tasks
    ADD COLUMN attempt int NOT NULL DEFAULT 0 CHECK (attempt >= 0);

CREATE TABLE task_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type varchar(64) NOT NULL,
    aggregate_id uuid NOT NULL REFERENCES import_tasks(id) ON DELETE CASCADE,
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);

CREATE INDEX idx_task_outbox_unpublished
    ON task_outbox(created_at)
    WHERE published_at IS NULL;

-- 升级时把已有待处理任务补入可靠投递箱。
INSERT INTO task_outbox (event_type, aggregate_id, payload)
SELECT 'document.parse', id, jsonb_build_object('task_id', id::text)
FROM import_tasks
WHERE status = 'pending' AND minio_object_key IS NOT NULL;
