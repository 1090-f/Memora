-- 文件作用：创建文档与检索相关表
-- 说明：
--   documents          - 文档表（支持手动创建、文件上传、URL 导入，记录处理管线状态及版本号）
--   document_chunks    - 文档分块表（分块内容、字符/Token 计数、全文检索向量 fts_vector）
--   document_vectors   - 文档向量表（存储 pgvector 向量嵌入，支持按模型和版本管理）
--   import_tasks       - 导入任务表（文件/URL 批量导入，支持去重策略与状态追踪）

-- 创建文档表，记录文档元数据、来源信息、处理管线状态及内容版本
CREATE TABLE documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id),
    directory_id uuid REFERENCES document_directories(id),
    title varchar(500) NOT NULL,
    content text,
    source_type varchar(20) NOT NULL CHECK (source_type IN ('manual', 'file', 'url')),
    source_url text,
    original_file_name varchar(500),
    file_size bigint CHECK (file_size >= 0),
    mime_type varchar(128),
    file_hash varchar(64),
    minio_bucket varchar(128),
    minio_object_key text,
    processing_status varchar(32) NOT NULL DEFAULT 'pending' CHECK (processing_status IN ('pending', 'parsing', 'cleaning', 'chunking', 'embedding', 'keyword_indexing', 'succeeded', 'failed')),
    failure_step varchar(64),
    failure_reason text,
    content_version int NOT NULL DEFAULT 1 CHECK (content_version > 0),
    chunk_version int NOT NULL DEFAULT 1 CHECK (chunk_version > 0),
    active_index_version int CHECK (active_index_version > 0),
    embedding_model_id uuid REFERENCES ai_model_configs(id),
    chunk_config_hash varchar(64),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

-- 创建索引，按用户和知识库查询文档并按更新时间降序排列
CREATE INDEX idx_documents_owner ON documents(user_id, knowledge_base_id, updated_at DESC) WHERE deleted_at IS NULL;
-- 创建索引，加速按处理状态筛选待处理文档
CREATE INDEX idx_documents_processing ON documents(processing_status, updated_at) WHERE deleted_at IS NULL;

-- 创建文档分块表，存储文档切分后的文本片段、字符/Token 计数及全文检索向量
CREATE TABLE document_chunks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id),
    document_id uuid NOT NULL REFERENCES documents(id),
    chunk_no int NOT NULL CHECK (chunk_no >= 0),
    content text NOT NULL,
    char_count int NOT NULL CHECK (char_count >= 0),
    token_count int NOT NULL CHECK (token_count >= 0),
    context_title text,
    heading_path jsonb,
    source_location jsonb,
    content_version int NOT NULL CHECK (content_version > 0),
    chunk_version int NOT NULL CHECK (chunk_version > 0),
    index_version int NOT NULL CHECK (index_version > 0),
    chunk_config_hash varchar(64) NOT NULL,
    fts_tokens text NOT NULL,
    fts_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple', fts_tokens)) STORED,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(document_id, index_version, chunk_no)
);

-- 创建 GIN 索引，加速基于 tsvector 的全文检索查询
CREATE INDEX idx_chunks_fts ON document_chunks USING GIN(fts_vector);
-- 创建组合索引，按用户、知识库、文档和版本号筛选分块
CREATE INDEX idx_chunks_filter ON document_chunks(user_id, knowledge_base_id, document_id, index_version);

-- 创建文档向量表，存储 pgvector 向量嵌入，按模型和版本管理
CREATE TABLE document_vectors (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id),
    document_id uuid NOT NULL REFERENCES documents(id),
    chunk_id uuid NOT NULL REFERENCES document_chunks(id),
    index_version int NOT NULL CHECK (index_version > 0),
    embedding_model_id uuid NOT NULL REFERENCES ai_model_configs(id),
    embedding_dim int NOT NULL CHECK (embedding_dim > 0),
    embedding vector NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(chunk_id, embedding_model_id, index_version)
);

-- 创建组合索引，加速按用户、知识库、文档、版本和状态筛选向量
CREATE INDEX idx_vectors_filter ON document_vectors(user_id, knowledge_base_id, document_id, index_version, status);

-- 创建导入任务表，记录文件/URL 批量导入的进度、去重策略和状态
CREATE TABLE import_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    knowledge_base_id uuid NOT NULL REFERENCES knowledge_bases(id),
    target_directory_id uuid REFERENCES document_directories(id),
    source_type varchar(20) NOT NULL CHECK (source_type IN ('file', 'url')),
    file_name varchar(500),
    file_size bigint CHECK (file_size >= 0),
    mime_type varchar(128),
    source_url text,
    source_hash varchar(128),
    minio_bucket varchar(128),
    minio_object_key text,
    duplicate_policy varchar(20) NOT NULL DEFAULT 'create_new' CHECK (duplicate_policy IN ('create_new', 'skip')),
    status varchar(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'skipped')),
    current_step varchar(64),
    failure_reason text,
    document_id uuid REFERENCES documents(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    completed_at timestamptz
);

-- 创建索引，按用户和知识库查询导入任务并按创建时间降序排列
CREATE INDEX idx_import_tasks_owner ON import_tasks(user_id, knowledge_base_id, created_at DESC);
-- 创建索引，加速工作线程拉取待执行和正在执行的导入任务
CREATE INDEX idx_import_tasks_worker ON import_tasks(status, created_at) WHERE status IN ('pending', 'running');
