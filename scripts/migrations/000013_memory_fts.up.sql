-- 文件作用：为 memories 表增加全文检索字段，支持关键词多路召回
-- 说明：
--   fts_tokens  - 由应用层 NgramTokenizer 分词后写入的 token 串（空格分隔）
--   fts_vector  - 基于 fts_tokens 自动生成的 tsvector 列，使用 PostgreSQL simple 配置
--   同时为存量记忆数据回填 fts_tokens（直接使用 content 字段）

-- 新增 fts_tokens 列，存储分词后的 token 串
ALTER TABLE memories ADD COLUMN fts_tokens text NOT NULL DEFAULT '';

-- 新增 fts_vector 列，基于 fts_tokens 自动生成 tsvector（PostgreSQL 12+ 支持 GENERATED ALWAYS）
ALTER TABLE memories ADD COLUMN fts_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('simple', fts_tokens)) STORED;

-- 创建 GIN 索引加速全文检索
CREATE INDEX idx_memories_fts ON memories USING GIN(fts_vector);

-- 回填存量记忆数据的 fts_tokens（直接使用 content，应用层写入时会用 NgramTokenizer 分词）
-- 此处用 content 作为近似值，后续新写入的记忆会由应用层正确分词
UPDATE memories SET fts_tokens = content WHERE fts_tokens = '' AND deleted_at IS NULL;
