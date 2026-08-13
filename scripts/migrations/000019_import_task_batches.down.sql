DROP INDEX IF EXISTS idx_import_tasks_batch;

ALTER TABLE import_tasks
    DROP COLUMN IF EXISTS source_path,
    DROP COLUMN IF EXISTS batch_id;
