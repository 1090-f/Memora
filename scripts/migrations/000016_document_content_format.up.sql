-- 手工文档支持 Markdown 存储：documents.content_format 记录正文格式。
-- 默认 txt 保持既有行为；markdown 允许前端按 Markdown 渲染阅读模式。
ALTER TABLE documents ADD COLUMN content_format varchar(16) NOT NULL DEFAULT 'txt';
ALTER TABLE documents ADD CONSTRAINT documents_content_format_check CHECK (content_format IN ('txt', 'markdown'));
