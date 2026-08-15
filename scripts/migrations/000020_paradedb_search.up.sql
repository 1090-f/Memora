-- 文件作用：将文档与长期记忆的关键词检索统一迁移到 ParadeDB pg_search。
-- 中文检索由 ParadeDB 固定 bigram tokenizer 负责，应用层不再生成 N-gram token。

CREATE EXTENSION IF NOT EXISTS pg_search;

-- 删除原生 PostgreSQL FTS 索引和应用层预分词列。
DROP INDEX IF EXISTS idx_chunks_fts;
ALTER TABLE document_chunks DROP COLUMN IF EXISTS fts_vector;
ALTER TABLE document_chunks DROP COLUMN IF EXISTS fts_tokens;

DROP INDEX IF EXISTS idx_memories_fts;
ALTER TABLE memories DROP COLUMN IF EXISTS fts_vector;
ALTER TABLE memories DROP COLUMN IF EXISTS fts_tokens;

-- 每张表只创建一个 BM25 索引。过滤列一并纳入索引以便 ParadeDB 下推过滤。
CREATE INDEX idx_chunks_bm25
ON document_chunks
USING bm25 (
    id,
    (content::pdb.ngram(2, 2, 'positions=true')),
    user_id,
    knowledge_base_id,
    document_id,
    index_version
)
WITH (key_field = 'id');

CREATE INDEX idx_memories_bm25
ON memories
USING bm25 (
    id,
    (content::pdb.ngram(2, 2, 'positions=true')),
    user_id,
    (status::pdb.literal),
    (scope_type::pdb.literal),
    scope_id,
    deleted_at
)
WITH (key_field = 'id');
