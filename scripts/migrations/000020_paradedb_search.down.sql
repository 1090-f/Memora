-- 回滚到原生 PostgreSQL FTS 表结构，供回退到旧版本应用使用。

DROP INDEX IF EXISTS idx_chunks_bm25;
DROP INDEX IF EXISTS idx_memories_bm25;

ALTER TABLE document_chunks ADD COLUMN fts_tokens text NOT NULL DEFAULT '';
UPDATE document_chunks SET fts_tokens = content;
ALTER TABLE document_chunks ALTER COLUMN fts_tokens DROP DEFAULT;
ALTER TABLE document_chunks ADD COLUMN fts_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('simple', fts_tokens)) STORED;
CREATE INDEX idx_chunks_fts ON document_chunks USING GIN(fts_vector);

ALTER TABLE memories ADD COLUMN fts_tokens text NOT NULL DEFAULT '';
UPDATE memories SET fts_tokens = content;
ALTER TABLE memories ADD COLUMN fts_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('simple', fts_tokens)) STORED;
CREATE INDEX idx_memories_fts ON memories USING GIN(fts_vector);

-- pg_search 由部署镜像初始化且可能被其他对象使用，回滚本迁移时保留扩展本身。
