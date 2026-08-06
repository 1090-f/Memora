-- 000008_mcp_stdio.down.sql
-- 回滚 stdio 传输支持

-- 1. 移除 stdio 字段
ALTER TABLE mcp_servers
    DROP COLUMN IF EXISTS env_ciphertext,
    DROP COLUMN IF EXISTS args_ciphertext,
    DROP COLUMN IF EXISTS command;

-- 2. url 恢复非空约束
UPDATE mcp_servers SET url = '' WHERE url IS NULL;
ALTER TABLE mcp_servers
    ALTER COLUMN url SET NOT NULL;

-- 3. 恢复 transport 仅允许 streamable_http
ALTER TABLE mcp_servers
    DROP CONSTRAINT IF EXISTS mcp_servers_transport_check;

ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_transport_check
    CHECK (transport = 'streamable_http');
