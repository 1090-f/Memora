-- 回滚 agent_events 表
DROP INDEX IF EXISTS idx_agent_events_run_id_sequence_unique;
DROP INDEX IF EXISTS idx_agent_events_run_id_sequence;
DROP TABLE IF EXISTS agent_events;