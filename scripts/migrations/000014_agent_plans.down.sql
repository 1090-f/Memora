-- 删除表（只删除本次迁移创建的表）
-- 注意：agent_plans 和 agent_plan_steps 表在 000005_conversation_agent 中创建，不应在此删除
DROP TABLE IF EXISTS agent_plan_execution_logs;
