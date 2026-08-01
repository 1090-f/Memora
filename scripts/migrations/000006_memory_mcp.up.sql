CREATE TABLE memories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    memory_type varchar(20) NOT NULL CHECK (memory_type IN ('preference', 'project', 'decision', 'goal', 'fact', 'progress')),
    scope_type varchar(20) NOT NULL CHECK (scope_type IN ('user', 'knowledge_base')),
    scope_id uuid REFERENCES knowledge_bases(id),
    content text NOT NULL,
    summary text,
    importance numeric(5,4) NOT NULL CHECK (importance BETWEEN 0 AND 1),
    content_hash varchar(64) NOT NULL,
    embedding vector NOT NULL,
    embedding_dim int NOT NULL CHECK (embedding_dim > 0),
    embedding_model_id uuid NOT NULL REFERENCES ai_model_configs(id),
    source_conversation_id uuid REFERENCES conversations(id),
    source_message_id uuid REFERENCES messages(id),
    source_agent_run_id uuid REFERENCES agent_runs(id),
    status varchar(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'deleted')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    last_accessed_at timestamptz,
    deleted_at timestamptz,
    CHECK ((scope_type = 'user' AND scope_id IS NULL) OR (scope_type = 'knowledge_base' AND scope_id IS NOT NULL))
);

CREATE INDEX idx_memories_scope ON memories(user_id, status, scope_type, scope_id);
CREATE UNIQUE INDEX uq_memories_content ON memories(user_id, scope_type, COALESCE(scope_id, '00000000-0000-0000-0000-000000000000'::uuid), content_hash) WHERE deleted_at IS NULL;

CREATE TABLE mcp_servers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    name varchar(128) NOT NULL,
    description text,
    transport varchar(32) NOT NULL DEFAULT 'streamable_http' CHECK (transport = 'streamable_http'),
    url text NOT NULL,
    headers_ciphertext bytea,
    auth_ciphertext bytea,
    auth_masked varchar(255),
    connect_timeout_ms int NOT NULL DEFAULT 5000 CHECK (connect_timeout_ms > 0),
    call_timeout_ms int NOT NULL DEFAULT 30000 CHECK (call_timeout_ms > 0),
    max_response_bytes int NOT NULL DEFAULT 1048576 CHECK (max_response_bytes > 0),
    enabled boolean NOT NULL DEFAULT true,
    connection_status varchar(20) NOT NULL DEFAULT 'unknown' CHECK (connection_status IN ('unknown', 'available', 'unavailable')),
    last_tested_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX uq_mcp_servers_user_name ON mcp_servers(user_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE mcp_tools (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id uuid NOT NULL REFERENCES mcp_servers(id),
    tool_name varchar(255) NOT NULL,
    description text,
    input_schema jsonb NOT NULL,
    schema_hash varchar(64) NOT NULL,
    read_only boolean NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    discovered_at timestamptz NOT NULL DEFAULT now(),
    last_checked_at timestamptz NOT NULL DEFAULT now(),
    schema_changed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(server_id, tool_name),
    CHECK (read_only = true OR enabled = false)
);

CREATE TABLE agent_mcp_tools (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_config_id uuid NOT NULL REFERENCES agent_configs(id),
    mcp_tool_id uuid NOT NULL REFERENCES mcp_tools(id),
    enabled boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(agent_config_id, mcp_tool_id)
);

ALTER TABLE tool_calls ADD CONSTRAINT fk_tool_calls_mcp_server FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id);
ALTER TABLE tool_calls ADD CONSTRAINT fk_tool_calls_mcp_tool FOREIGN KEY (mcp_tool_id) REFERENCES mcp_tools(id);
ALTER TABLE tool_calls ADD CONSTRAINT ck_tool_calls_mcp_identity CHECK (
    (tool_type = 'internal' AND mcp_server_id IS NULL AND mcp_tool_id IS NULL)
    OR (tool_type = 'mcp' AND mcp_server_id IS NOT NULL AND mcp_tool_id IS NOT NULL)
);
