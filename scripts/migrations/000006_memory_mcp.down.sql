-- 文件作用：删除记忆与 MCP 相关表（回滚）
-- 说明：移除 tool_calls 上的 MCP 约束，按依赖关系逆序删除 Agent-MCP 工具关联、MCP 工具、MCP 服务器、记忆

-- 移除工具调用表上的 MCP 身份检查约束
ALTER TABLE IF EXISTS tool_calls DROP CONSTRAINT IF EXISTS ck_tool_calls_mcp_identity;
-- 移除工具调用表上关联 MCP 工具的外键约束
ALTER TABLE IF EXISTS tool_calls DROP CONSTRAINT IF EXISTS fk_tool_calls_mcp_tool;
-- 移除工具调用表上关联 MCP 服务器的外键约束
ALTER TABLE IF EXISTS tool_calls DROP CONSTRAINT IF EXISTS fk_tool_calls_mcp_server;
-- 删除 Agent-MCP 工具关联表
DROP TABLE IF EXISTS agent_mcp_tools;
-- 删除 MCP 工具表
DROP TABLE IF EXISTS mcp_tools;
-- 删除 MCP 服务器表
DROP TABLE IF EXISTS mcp_servers;
-- 删除记忆表
DROP TABLE IF EXISTS memories;
