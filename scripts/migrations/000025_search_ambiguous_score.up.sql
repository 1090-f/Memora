-- 文件作用：检索配置新增"资料存在但依据不明确"判定阈值，实现知识充分性三态 AMBIGUOUS。
-- 语义：向量/混合检索中，当有效结果数量达标、但最高向量相似度仍低于本阈值时，
-- 知识状态判为 ambiguous（有依据但无法明确支持结论），介于 sufficient 与 insufficient 之间。
-- 该值需高于 min_vector_score 才有区分意义。

ALTER TABLE search_configs
    ADD COLUMN ambiguous_score numeric(8,6) NOT NULL DEFAULT 0.45
    CHECK (ambiguous_score BETWEEN 0 AND 1);

COMMENT ON COLUMN search_configs.ambiguous_score IS '最高向量相似度低于该值时判为 ambiguous（0~1，建议高于 min_vector_score）';
