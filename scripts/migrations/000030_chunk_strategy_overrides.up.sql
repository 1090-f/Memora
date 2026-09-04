ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS chunk_strategy VARCHAR(32);

ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS chunk_strategy VARCHAR(32);

ALTER TABLE knowledge_bases
    ADD CONSTRAINT knowledge_bases_chunk_strategy_check
    CHECK (chunk_strategy IS NULL OR chunk_strategy IN ('structured', 'paragraph', 'recursive_fallback', 'auto'));

ALTER TABLE documents
    ADD CONSTRAINT documents_chunk_strategy_check
    CHECK (chunk_strategy IS NULL OR chunk_strategy IN ('structured', 'paragraph', 'recursive_fallback', 'auto'));

COMMENT ON COLUMN knowledge_bases.chunk_strategy IS '知识库级分块策略覆盖；NULL 表示继承环境配置';
COMMENT ON COLUMN documents.chunk_strategy IS '文档级分块策略覆盖；优先于知识库和环境配置';
