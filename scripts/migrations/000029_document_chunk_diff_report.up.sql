ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS chunk_diff_report JSONB;

COMMENT ON COLUMN documents.chunk_diff_report IS '旧分块器与 Canonical 候选策略的最近一次影子差异报告';
