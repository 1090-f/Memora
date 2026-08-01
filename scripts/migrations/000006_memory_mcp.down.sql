ALTER TABLE IF EXISTS tool_calls DROP CONSTRAINT IF EXISTS ck_tool_calls_mcp_identity;
ALTER TABLE IF EXISTS tool_calls DROP CONSTRAINT IF EXISTS fk_tool_calls_mcp_tool;
ALTER TABLE IF EXISTS tool_calls DROP CONSTRAINT IF EXISTS fk_tool_calls_mcp_server;
DROP TABLE IF EXISTS agent_mcp_tools;
DROP TABLE IF EXISTS mcp_tools;
DROP TABLE IF EXISTS mcp_servers;
DROP TABLE IF EXISTS memories;
