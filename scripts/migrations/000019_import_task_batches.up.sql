-- 文件夹批量导入：同一上传批次标识与归档内源路径。
ALTER TABLE import_tasks
    ADD COLUMN IF NOT EXISTS batch_id uuid,
    ADD COLUMN IF NOT EXISTS source_path text;

CREATE INDEX IF NOT EXISTS idx_import_tasks_batch
    ON import_tasks(user_id, knowledge_base_id, batch_id, created_at DESC);
