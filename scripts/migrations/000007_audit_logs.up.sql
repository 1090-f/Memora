-- 文件作用：创建审计日志表
-- 说明：持久化审计事件（登录、登出、资料修改、密码修改等），便于安全审计与问题追溯

-- 创建审计日志表
CREATE TABLE audit_logs (
    id bigserial PRIMARY KEY,
    action varchar(64) NOT NULL,
    actor_id varchar(64),
    resource varchar(128),
    request_id varchar(64),
    trace_id varchar(64),
    outcome varchar(16) NOT NULL CHECK (outcome IN ('succeeded', 'denied', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- 创建索引，加速按时间、动作和操作者的查询
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_actor_id ON audit_logs(actor_id);
