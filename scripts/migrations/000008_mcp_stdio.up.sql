-- 000008_mcp_stdio.up.sql
-- 扩展 mcp_servers 表以支持 stdio 传输方式

-- 1. 扩展 transport 枚举，允许 stdio
ALTER TABLE mcp_servers
    DROP CONSTRAINT IF EXISTS mcp_servers_transport_check;

ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_transport_check
    CHECK (transport IN ('streamable_http', 'stdio'));

-- 2. url 允许为空（stdio server 无 url）
ALTER TABLE mcp_servers
    ALTER COLUMN url DROP NOT NULL;

-- 3. 新增 stdio 字段（command/args/env 整体加密存储）
ALTER TABLE mcp_servers
    ADD COLUMN IF NOT EXISTS command varchar(255),
    ADD COLUMN IF NOT EXISTS args_ciphertext bytea,
    ADD COLUMN IF NOT EXISTS env_ciphertext bytea;
