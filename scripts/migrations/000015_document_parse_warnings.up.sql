-- 文档解析警告（如 unresolved 图片），成功加工后由 Worker 回写
ALTER TABLE documents ADD COLUMN IF NOT EXISTS parse_warnings jsonb;
