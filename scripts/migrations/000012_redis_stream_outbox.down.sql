DROP TABLE IF EXISTS task_outbox;
ALTER TABLE import_tasks DROP COLUMN IF EXISTS attempt;
