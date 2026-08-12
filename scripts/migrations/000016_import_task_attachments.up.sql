-- 导入任务附件映射（zip 导入：zip 内相对路径 → MinIO object key）
ALTER TABLE import_tasks ADD COLUMN IF NOT EXISTS attachments jsonb;

-- 说明：zip 导入时主文档按既有流程落 import_tasks 主行，
-- 附属图片存于 MinIO，attachments 记录相对路径与对象 key 的映射，
-- 供 Worker 解析 Markdown 图片引用时读取。
