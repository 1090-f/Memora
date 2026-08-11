-- Agent Plan Execution Logs 表：记录计划执行的审计日志
-- 注意：agent_plans 和 agent_plan_steps 表已在 000005_conversation_agent 中创建
CREATE TABLE IF NOT EXISTS agent_plan_execution_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES agent_plans(id) ON DELETE CASCADE,
    step_no INT,
    event_type VARCHAR(50) NOT NULL,
    old_status VARCHAR(20),
    new_status VARCHAR(20),
    message TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 创建索引
CREATE INDEX idx_agent_plan_execution_logs_plan_id ON agent_plan_execution_logs(plan_id);
CREATE INDEX idx_agent_plan_execution_logs_created_at ON agent_plan_execution_logs(created_at);
