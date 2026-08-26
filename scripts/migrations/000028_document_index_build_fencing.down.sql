DROP INDEX IF EXISTS idx_documents_index_build_owner;

ALTER TABLE documents
    DROP COLUMN IF EXISTS index_build_started_at,
    DROP COLUMN IF EXISTS index_build_version,
    DROP COLUMN IF EXISTS index_build_owner;
