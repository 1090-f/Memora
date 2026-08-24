ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_chunk_strategy_check;
ALTER TABLE knowledge_bases DROP CONSTRAINT IF EXISTS knowledge_bases_chunk_strategy_check;
ALTER TABLE documents DROP COLUMN IF EXISTS chunk_strategy;
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS chunk_strategy;
