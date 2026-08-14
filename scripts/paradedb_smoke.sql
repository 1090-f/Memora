\set ON_ERROR_STOP on

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_search;

CREATE TABLE document_chunks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    knowledge_base_id uuid NOT NULL,
    document_id uuid NOT NULL,
    index_version int NOT NULL,
    content text NOT NULL,
    fts_tokens text NOT NULL DEFAULT '',
    fts_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple', fts_tokens)) STORED
);
CREATE INDEX idx_chunks_fts ON document_chunks USING GIN(fts_vector);

DROP INDEX idx_chunks_fts;
ALTER TABLE document_chunks DROP COLUMN fts_vector;
ALTER TABLE document_chunks DROP COLUMN fts_tokens;
CREATE INDEX idx_chunks_bm25 ON document_chunks USING bm25 (
    id,
    (content::pdb.ngram(2, 2, 'positions=true')),
    user_id,
    knowledge_base_id,
    document_id,
    index_version
) WITH (key_field = 'id');

INSERT INTO document_chunks (user_id, knowledge_base_id, document_id, index_version, content)
VALUES
    ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000101', 1, '胡智敏负责 Go 后端开发'),
    ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000102', 1, '胡智负责开发，智敏负责检索'),
    ('00000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000103', 1, '胡适讨论人工智能'),
    ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000104', 1, '胡智敏属于其他用户');

DO $$
DECLARE
    matched int;
BEGIN
    SELECT count(*) INTO matched
    FROM document_chunks
    WHERE content ### '胡智敏'
      AND user_id = '00000000-0000-0000-0000-000000000001';
    IF matched <> 1 THEN
        RAISE EXCEPTION 'exact phrase expected 1 row, got %', matched;
    END IF;

    SELECT count(*) INTO matched
    FROM document_chunks
    WHERE content &&& '胡智敏'
      AND user_id = '00000000-0000-0000-0000-000000000001';
    IF matched <> 2 THEN
        RAISE EXCEPTION 'conjunction expected 2 rows, got %', matched;
    END IF;

    SELECT count(*) INTO matched
    FROM document_chunks
    WHERE content ||| '胡智敏'
      AND user_id = '00000000-0000-0000-0000-000000000001';
    IF matched <> 2 THEN
        RAISE EXCEPTION 'disjunction expected 2 rows without unigram noise, got %', matched;
    END IF;
END
$$;

SELECT id, content, pdb.score(id) AS score
FROM document_chunks
WHERE content ||| '胡智敏'
  AND user_id = '00000000-0000-0000-0000-000000000001'
ORDER BY pdb.score(id) DESC, id;

CREATE TABLE memories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    content text NOT NULL,
    status text NOT NULL,
    scope_type text NOT NULL,
    scope_id uuid,
    deleted_at timestamptz,
    fts_tokens text NOT NULL DEFAULT '',
    fts_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple', fts_tokens)) STORED
);
CREATE INDEX idx_memories_fts ON memories USING GIN(fts_vector);

DROP INDEX idx_memories_fts;
ALTER TABLE memories DROP COLUMN fts_vector;
ALTER TABLE memories DROP COLUMN fts_tokens;
CREATE INDEX idx_memories_bm25 ON memories USING bm25 (
    id,
    (content::pdb.ngram(2, 2, 'positions=true')),
    user_id,
    (status::pdb.literal),
    (scope_type::pdb.literal),
    scope_id,
    deleted_at
) WITH (key_field = 'id');

INSERT INTO memories (user_id, content, status, scope_type)
VALUES
    ('00000000-0000-0000-0000-000000000001', '记住胡智敏负责后端开发', 'active', 'user'),
    ('00000000-0000-0000-0000-000000000001', '人工智能项目', 'active', 'user'),
    ('00000000-0000-0000-0000-000000000002', '胡智敏属于其他用户', 'active', 'user');

DO $$
DECLARE
    matched int;
BEGIN
    SELECT count(*) INTO matched
    FROM memories
    WHERE content ||| '胡智敏'
      AND user_id = '00000000-0000-0000-0000-000000000001'
      AND status = 'active'
      AND deleted_at IS NULL;
    IF matched <> 1 THEN
        RAISE EXCEPTION 'memory search expected 1 row, got %', matched;
    END IF;
END
$$;
