-- 000027_mcp_network_required.up.sql
-- 为 mcp_servers 增加 network_required 字段，控制 MCP 工具是否受知识库"联网"开关约束。
-- 默认 false（本地 stdio 工具无需联网）；已存在的 streamable_http 服务按原行为标记为 true。

ALTER TABLE mcp_servers
    ADD COLUMN IF NOT EXISTS network_required boolean NOT NULL DEFAULT false;

-- 存量 streamable_http 服务需要联网，保持原有"受联网开关约束"的行为
UPDATE mcp_servers
SET network_required = true
WHERE transport = 'streamable_http' AND deleted_at IS NULL;