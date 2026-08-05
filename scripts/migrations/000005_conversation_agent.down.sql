-- 文件作用：删除会话与 Agent 执行相关表（回滚）
-- 说明：按依赖关系逆序删除工具调用、计划步骤、计划、Agent 运行、消息、会话

-- 删除工具调用表
DROP TABLE IF EXISTS tool_calls;
-- 删除 Agent 计划步骤表
DROP TABLE IF EXISTS agent_plan_steps;
-- 删除 Agent 计划表
DROP TABLE IF EXISTS agent_plans;
-- 移除消息表上关联 Agent 运行记录的外键约束
ALTER TABLE IF EXISTS messages DROP CONSTRAINT IF EXISTS fk_messages_agent_run;
-- 删除 Agent 运行表
DROP TABLE IF EXISTS agent_runs;
-- 删除消息表
DROP TABLE IF EXISTS messages;
-- 删除会话表
DROP TABLE IF EXISTS conversations;
