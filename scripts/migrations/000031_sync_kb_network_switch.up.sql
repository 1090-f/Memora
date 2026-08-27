-- 000028_sync_kb_network_switch.up.sql
-- 同步存量数据：把 knowledge_bases.network_enabled 回填到 agent_configs.network_enabled。
-- 历史版本创建知识库时 agent_configs.network_enabled 被硬编码为 false，
-- 而前端开关只写 knowledge_bases.network_enabled，两者从未同步，
-- 导致运行时的联网开关恒为 false，MCP 等联网工具被误拦截（NETWORK_DISABLED）。
-- 本迁移让存量知识库立即按用户已设置的开关生效，无需等待下次保存。

UPDATE agent_configs a
SET network_enabled = k.network_enabled
FROM knowledge_bases k
WHERE a.knowledge_base_id = k.id
  AND k.deleted_at IS NULL
  AND a.network_enabled <> k.network_enabled;