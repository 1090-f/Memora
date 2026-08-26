ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS index_build_owner VARCHAR(64),
    ADD COLUMN IF NOT EXISTS index_build_version INTEGER,
    ADD COLUMN IF NOT EXISTS index_build_started_at TIMESTAMPTZ;

COMMENT ON COLUMN documents.index_build_owner IS '当前候选索引构建任务 ID，用作 fencing owner';
COMMENT ON COLUMN documents.index_build_version IS '当前构建任务持有的候选索引版本';
COMMENT ON COLUMN documents.index_build_started_at IS '当前候选索引构建开始时间，用于过期接管';

CREATE INDEX IF NOT EXISTS idx_documents_index_build_owner
    ON documents(index_build_owner)
    WHERE index_build_owner IS NOT NULL AND deleted_at IS NULL;
