-- 000009_mcp_stdio_cwd.up.sql
-- 持久化 stdio MCP Server 的工作目录配置。

ALTER TABLE mcp_servers
    ADD COLUMN IF NOT EXISTS cwd varchar(1024);
