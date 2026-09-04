-- 000028_sync_kb_network_switch.down.sql
-- 回滚数据回填：恢复历史行为，agent_configs.network_enabled 统一重置为 false。
-- 注意：回滚会丢弃用户后续通过同步机制写入的开关值，仅用于还原迁移前的旧状态。

UPDATE agent_configs a
SET network_enabled = false
FROM knowledge_bases k
WHERE a.knowledge_base_id = k.id
  AND k.deleted_at IS NULL
  AND a.network_enabled <> false;