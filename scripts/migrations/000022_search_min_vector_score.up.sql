-- 文件作用：检索配置新增向量相似度最低阈值，用于知识充分性判断。
-- 向量/混合检索在召回层过滤低于该分数的结果，避免低质量结果被判定为 sufficient。

ALTER TABLE search_configs
    ADD COLUMN IF NOT EXISTS min_vector_score numeric(8,6) NOT NULL DEFAULT 0.3
    CHECK (min_vector_score BETWEEN 0 AND 1);

COMMENT ON COLUMN search_configs.min_vector_score IS '向量检索最低相似度阈值（0~1），0 表示不启用过滤';
