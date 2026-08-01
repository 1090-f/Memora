CREATE TABLE conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id),
    title varchar(255),
    status varchar(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_conversations_owner ON conversations(user_id, knowledge_base_id, updated_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id uuid NOT NULL REFERENCES conversations(id),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id),
    agent_run_id uuid,
    role varchar(20) NOT NULL CHECK (role IN ('user', 'assistant')),
    content text NOT NULL,
    citations jsonb,
    model_config_id uuid REFERENCES ai_model_configs(id),
    input_tokens int CHECK (input_tokens >= 0),
    output_tokens int CHECK (output_tokens >= 0),
    response_time_ms bigint CHECK (response_time_ms >= 0),
    status varchar(20) NOT NULL CHECK (status IN ('streaming', 'completed', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_conversation_messages ON messages(conversation_id, created_at);

CREATE TABLE agent_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id),
    conversation_id uuid NOT NULL REFERENCES conversations(id),
    user_message_id uuid NOT NULL REFERENCES messages(id),
    assistant_message_id uuid REFERENCES messages(id),
    agent_config_id uuid NOT NULL REFERENCES agent_configs(id),
    retry_of_run_id uuid REFERENCES agent_runs(id),
    query text NOT NULL,
    execution_mode varchar(20) CHECK (execution_mode IN ('react', 'plan_execute')),
    router_reason_summary varchar(1000),
    router_confidence numeric(5,4) CHECK (router_confidence BETWEEN 0 AND 1),
    router_fallback_used boolean NOT NULL DEFAULT false,
    knowledge_status varchar(20) CHECK (knowledge_status IN ('sufficient', 'insufficient', 'ambiguous')),
    execution_trace jsonb,
    replan_count int NOT NULL DEFAULT 0 CHECK (replan_count BETWEEN 0 AND 1),
    reviewer_result varchar(32) CHECK (reviewer_result IN ('pass', 'needs_attention', 'failed')),
    reviewer_summary text,
    memory_used_count int NOT NULL DEFAULT 0 CHECK (memory_used_count >= 0),
    status varchar(20) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    input_tokens int NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens int NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    total_tokens int NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
    duration_ms bigint CHECK (duration_ms >= 0),
    final_result text,
    error_code varchar(64),
    error_message text,
    started_at timestamptz,
    ended_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE messages ADD CONSTRAINT fk_messages_agent_run FOREIGN KEY (agent_run_id) REFERENCES agent_runs(id);
CREATE INDEX idx_agent_runs_owner ON agent_runs(user_id, knowledge_base_id, created_at DESC);
CREATE INDEX idx_agent_runs_worker ON agent_runs(status, created_at) WHERE status IN ('queued', 'running');

CREATE TABLE agent_plans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_run_id uuid NOT NULL REFERENCES agent_runs(id),
    version int NOT NULL CHECK (version BETWEEN 1 AND 2),
    goal text NOT NULL,
    completion_criteria jsonb,
    status varchar(20) NOT NULL CHECK (status IN ('pending', 'executing', 'replanning', 'reviewing', 'completed', 'failed', 'cancelled')),
    is_current boolean NOT NULL DEFAULT true,
    replan_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    UNIQUE(agent_run_id, version)
);

CREATE UNIQUE INDEX uq_current_plan ON agent_plans(agent_run_id) WHERE is_current = true;

CREATE TABLE agent_plan_steps (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id uuid NOT NULL REFERENCES agent_plans(id),
    step_no int NOT NULL CHECK (step_no BETWEEN 1 AND 5),
    title varchar(255) NOT NULL,
    description text,
    depends_on jsonb NOT NULL DEFAULT '[]'::jsonb,
    recommended_tool varchar(128),
    completion_criteria text,
    status varchar(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'skipped', 'cancelled')),
    input_summary text,
    output_summary text,
    error_code varchar(64),
    error_message text,
    started_at timestamptz,
    ended_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(plan_id, step_no)
);

CREATE TABLE tool_calls (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_run_id uuid NOT NULL REFERENCES agent_runs(id),
    plan_step_id uuid REFERENCES agent_plan_steps(id),
    react_round_no int CHECK (react_round_no > 0),
    tool_name varchar(128) NOT NULL,
    tool_type varchar(20) NOT NULL CHECK (tool_type IN ('internal', 'mcp')),
    mcp_server_id uuid,
    mcp_tool_id uuid,
    status varchar(20) NOT NULL CHECK (status IN ('running', 'succeeded', 'failed', 'timeout', 'cancelled')),
    arguments_redacted jsonb,
    input_summary text,
    output_summary text,
    result_meta jsonb,
    response_bytes bigint CHECK (response_bytes >= 0),
    is_truncated boolean NOT NULL DEFAULT false,
    error_code varchar(64),
    error_message text,
    duration_ms bigint CHECK (duration_ms >= 0),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    CHECK ((tool_type = 'internal' AND mcp_server_id IS NULL AND mcp_tool_id IS NULL) OR tool_type = 'mcp')
);

CREATE INDEX idx_tool_calls_run ON tool_calls(agent_run_id, started_at);
