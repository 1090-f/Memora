-- 000009_mcp_stdio_cwd.down.sql
-- 回滚 stdio MCP Server 的工作目录配置。

ALTER TABLE mcp_servers
    DROP COLUMN IF EXISTS cwd;
