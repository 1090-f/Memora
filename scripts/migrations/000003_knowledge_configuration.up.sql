CREATE TABLE ai_model_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    model_type varchar(20) NOT NULL CHECK (model_type IN ('chat', 'embedding', 'reranker')),
    provider varchar(64) NOT NULL,
    name varchar(128) NOT NULL,
    base_url text NOT NULL,
    api_key_ciphertext bytea,
    api_key_masked varchar(64),
    timeout_seconds int NOT NULL DEFAULT 60 CHECK (timeout_seconds > 0),
    retry_times int NOT NULL DEFAULT 2 CHECK (retry_times BETWEEN 0 AND 10),
    max_tokens int CHECK (max_tokens > 0),
    temperature numeric(4,3) CHECK (temperature BETWEEN 0 AND 2),
    vector_dimension int CHECK (vector_dimension > 0),
    supports_tool_calling boolean NOT NULL DEFAULT false,
    supports_streaming boolean NOT NULL DEFAULT false,
    is_default boolean NOT NULL DEFAULT false,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_model_configs_user_type ON ai_model_configs(user_id, model_type, updated_at DESC) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_model_configs_default ON ai_model_configs(user_id, model_type) WHERE is_default = true AND deleted_at IS NULL;

CREATE TABLE knowledge_bases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    name varchar(128) NOT NULL,
    description text,
    icon varchar(255),
    default_language varchar(32) NOT NULL DEFAULT 'zh-CN',
    qa_enabled boolean NOT NULL DEFAULT true,
    agent_enabled boolean NOT NULL DEFAULT true,
    network_enabled boolean NOT NULL DEFAULT false,
    default_chat_model_id uuid REFERENCES ai_model_configs(id),
    default_embedding_model_id uuid REFERENCES ai_model_configs(id),
    default_reranker_model_id uuid REFERENCES ai_model_configs(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_kb_user_updated ON knowledge_bases(user_id, updated_at DESC) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_kb_user_name ON knowledge_bases(user_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE document_directories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id),
    parent_id uuid REFERENCES document_directories(id),
    name varchar(128) NOT NULL,
    depth smallint NOT NULL CHECK (depth BETWEEN 1 AND 5),
    sort_order int NOT NULL DEFAULT 0,
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX idx_directories_tree ON document_directories(user_id, knowledge_base_id, parent_id, sort_order) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_directories_default ON document_directories(knowledge_base_id) WHERE is_default = true AND deleted_at IS NULL;

CREATE TABLE search_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_base_id uuid NOT NULL UNIQUE REFERENCES knowledge_bases(id),
    keyword_top_k int NOT NULL DEFAULT 30 CHECK (keyword_top_k > 0),
    vector_top_k int NOT NULL DEFAULT 30 CHECK (vector_top_k > 0),
    rrf_k int NOT NULL DEFAULT 60 CHECK (rrf_k > 0),
    rrf_top_k int NOT NULL DEFAULT 20 CHECK (rrf_top_k > 0),
    reranker_top_k int NOT NULL DEFAULT 8 CHECK (reranker_top_k > 0),
    reranker_threshold numeric(8,6) CHECK (reranker_threshold BETWEEN 0 AND 1),
    minimum_effective_results int NOT NULL DEFAULT 1 CHECK (minimum_effective_results > 0),
    reranker_model_id uuid REFERENCES ai_model_configs(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE agent_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_base_id uuid NOT NULL UNIQUE REFERENCES knowledge_bases(id),
    name varchar(128) NOT NULL DEFAULT 'Default Agent',
    system_prompt text,
    chat_model_id uuid NOT NULL REFERENCES ai_model_configs(id),
    max_react_rounds int NOT NULL DEFAULT 8 CHECK (max_react_rounds > 0),
    max_plan_steps int NOT NULL DEFAULT 5 CHECK (max_plan_steps BETWEEN 1 AND 5),
    max_replans int NOT NULL DEFAULT 1 CHECK (max_replans BETWEEN 0 AND 1),
    reviewer_runs int NOT NULL DEFAULT 1 CHECK (reviewer_runs BETWEEN 0 AND 1),
    max_tool_calls int NOT NULL DEFAULT 10 CHECK (max_tool_calls > 0),
    max_document_read_tokens int NOT NULL DEFAULT 6000 CHECK (max_document_read_tokens > 0),
    max_tool_result_bytes int NOT NULL DEFAULT 1048576 CHECK (max_tool_result_bytes > 0),
    max_run_seconds int NOT NULL DEFAULT 300 CHECK (max_run_seconds > 0),
    network_enabled boolean NOT NULL DEFAULT false,
    memory_enabled boolean NOT NULL DEFAULT true,
    memory_top_k int NOT NULL DEFAULT 8 CHECK (memory_top_k > 0),
    show_execution_status boolean NOT NULL DEFAULT true,
    status varchar(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_agent_configs_user ON agent_configs(user_id, updated_at DESC);
