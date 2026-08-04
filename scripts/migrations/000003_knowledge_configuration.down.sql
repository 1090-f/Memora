-- 文件作用：删除知识库配置相关表（回滚）
-- 说明：按依赖关系逆序删除 Agent 配置、检索配置、文档目录、知识库、AI 模型配置

-- 删除 Agent 配置表
DROP TABLE IF EXISTS agent_configs;
-- 删除检索配置表
DROP TABLE IF EXISTS search_configs;
-- 删除文档目录表
DROP TABLE IF EXISTS document_directories;
-- 删除知识库表
DROP TABLE IF EXISTS knowledge_bases;
-- 删除 AI 模型配置表
DROP TABLE IF EXISTS ai_model_configs;
