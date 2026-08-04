-- 文件作用：启用 PostgreSQL 扩展
-- 说明：启用 pgcrypto 扩展（提供加密函数，用于 UUID 生成和密码哈希）和 vector 扩展（pgvector，用于向量相似度检索）

-- 启用 pgcrypto 扩展，提供加密函数（用于 UUID 生成和密码哈希）
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- 启用 pgvector 扩展，提供向量类型和向量相似度检索操作符
CREATE EXTENSION IF NOT EXISTS vector;
