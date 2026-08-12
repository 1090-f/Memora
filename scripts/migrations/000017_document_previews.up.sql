-- 文档视觉预览派生产物及异步任务状态。
CREATE TABLE document_previews (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    document_id uuid NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    content_version int NOT NULL CHECK (content_version > 0),
    preview_type varchar(20) NOT NULL CHECK (preview_type IN ('pdf', 'table')),
    status varchar(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'ready', 'failed', 'unsupported')),
    render_hash varchar(64) NOT NULL,
    renderer varchar(64) NOT NULL,
    renderer_version varchar(128) NOT NULL,
    object_key text,
    manifest_key text,
    media_type varchar(128),
    object_size bigint CHECK (object_size IS NULL OR object_size >= 0),
    attempt int NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    error_code varchar(64),
    error_message text,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(document_id, content_version, preview_type, render_hash)
);

CREATE INDEX idx_document_previews_lookup
    ON document_previews(document_id, content_version, preview_type, updated_at DESC);

CREATE INDEX idx_document_previews_worker
    ON document_previews(status, created_at)
    WHERE status IN ('pending', 'processing');

-- task_outbox 从“仅导入任务”泛化为按 event_type 路由的可靠任务投递箱。
ALTER TABLE task_outbox DROP CONSTRAINT IF EXISTS task_outbox_aggregate_id_fkey;
