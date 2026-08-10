-- 000010_uuid_primary_key_defaults.up.sql
-- 修复知识库初始化事务涉及表的 UUID 主键默认值。
-- golang-migrate 会记录该版本，后续启动检测到已执行后会自动跳过。

ALTER TABLE knowledge_bases
    ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE document_directories
    ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE search_configs
    ALTER COLUMN id SET DEFAULT gen_random_uuid();

ALTER TABLE agent_configs
    ALTER COLUMN id SET DEFAULT gen_random_uuid();
