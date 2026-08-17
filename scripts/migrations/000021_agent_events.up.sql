-- 文件作用：创建 agent_events 表，持久化 Agent 运行的所有中间过程事件。
-- 用于切换会话后恢复实时运行状态，以及刷新页面后重建完整的中间过程展示。
--
-- 设计原则：
--   - 每条记录对应一个 AgentEvent（SSE 推送的原子事件单元）
--   - run_id + sequence 唯一索引防止重复写入
--   - CASCADE DELETE 跟随 agent_runs 清理
--   - 仅追加、不更新，确保事件日志的不可变性

CREATE TABLE agent_events (
    id          BIGSERIAL       PRIMARY KEY,                                   -- 自增主键，保证写入顺序
    run_id      UUID            NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,  -- 所属运行 ID，级联删除
    sequence    BIGINT          NOT NULL,                                       -- 事件序号（单调递增，同 run 内唯一）
    event_type  VARCHAR(64)     NOT NULL,                                       -- 事件类型（agent.run.started 等）
    timestamp   TIMESTAMPTZ     NOT NULL DEFAULT NOW(),                         -- 事件发生时间
    data        JSONB           NOT NULL DEFAULT '{}',                          -- 事件附加数据（原始 JSON）
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()                          -- 记录创建时间
);

-- 复合索引：按 run_id 查询所有事件 + 按 sequence 过滤已消费事件
CREATE INDEX idx_agent_events_run_id_sequence ON agent_events(run_id, sequence);

-- 唯一约束：防止 events 重入导致的重复持久化
CREATE UNIQUE INDEX idx_agent_events_run_id_sequence_unique ON agent_events(run_id, sequence);