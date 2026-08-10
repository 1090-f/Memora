-- 000011_rag_uuid_primary_key_defaults.up.sql
-- 补齐 RAG 导入与索引链路表的 UUID 主键默认值。
-- golang-migrate 会记录该版本，执行成功后的启动将自动跳过。

ALTER TABLE import_tasks
    ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE document_chunks
    ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE document_vectors
    ALTER COLUMN id SET DEFAULT gen_random_uuid();
