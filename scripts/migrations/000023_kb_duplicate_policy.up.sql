-- 文件作用：知识库级重复策略，替代每次导入请求中的 duplicate_policy 参数。
-- 取值：skip（重复内容跳过，默认）/ create_new（重复内容创建新文档）。

ALTER TABLE knowledge_bases
    ADD COLUMN IF NOT EXISTS duplicate_policy varchar(20) NOT NULL DEFAULT 'skip';

COMMENT ON COLUMN knowledge_bases.duplicate_policy IS '文档导入重复处理策略：skip / create_new';
