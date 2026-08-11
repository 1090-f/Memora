ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_content_format_check;
ALTER TABLE documents DROP COLUMN IF EXISTS content_format;
