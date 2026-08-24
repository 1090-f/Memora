DROP VIEW IF EXISTS model_binding_migration_anomalies;

DROP INDEX IF EXISTS idx_agent_run_chat_model;
DROP INDEX IF EXISTS idx_conversation_chat_model;
DROP INDEX IF EXISTS idx_kb_embedding_model;

ALTER TABLE agent_runs DROP COLUMN IF EXISTS chat_model_id;
ALTER TABLE conversations DROP COLUMN IF EXISTS chat_model_id;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS embedding_model_id;

-- 旧列可能已有 NULL；仅在数据允许时由运维手动恢复 NOT NULL。
