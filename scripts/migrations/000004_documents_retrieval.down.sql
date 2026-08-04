-- 文件作用：删除文档与检索相关表（回滚）
-- 说明：按依赖关系逆序删除导入任务、文档向量、文档分块、文档表

-- 删除导入任务表
DROP TABLE IF EXISTS import_tasks;
-- 删除文档向量表
DROP TABLE IF EXISTS document_vectors;
-- 删除文档分块表
DROP TABLE IF EXISTS document_chunks;
-- 删除文档表
DROP TABLE IF EXISTS documents;
