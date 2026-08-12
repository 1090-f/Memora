DELETE FROM task_outbox WHERE event_type = 'document.preview.render';
DROP TABLE IF EXISTS document_previews;

-- 恢复迁移前约束；此时 aggregate_id 只剩 import_tasks id。
ALTER TABLE task_outbox
    ADD CONSTRAINT task_outbox_aggregate_id_fkey
    FOREIGN KEY (aggregate_id) REFERENCES import_tasks(id) ON DELETE CASCADE;
