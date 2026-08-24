-- 模型绑定生命周期兼容迁移：新增新字段、按真实索引优先回填，旧字段暂留供核验后清理。

ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS embedding_model_id uuid REFERENCES ai_model_configs(id);
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS chat_model_id uuid REFERENCES ai_model_configs(id);
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS chat_model_id uuid REFERENCES ai_model_configs(id);

-- AgentConfig 不再决定 Chat 模型。兼容期保留旧列，但解除非空约束。
ALTER TABLE agent_configs ALTER COLUMN chat_model_id DROP NOT NULL;

-- 1. 当前生效索引的实际向量记录优先。仅在唯一模型明确时自动回填。
WITH active_vector_models AS (
    SELECT d.knowledge_base_id, min(v.embedding_model_id::text)::uuid AS embedding_model_id
    FROM documents d
    JOIN document_vectors v
      ON v.document_id = d.id
     AND v.index_version = d.active_index_version
     AND v.status = 'ready'
    WHERE d.deleted_at IS NULL
      AND d.active_index_version IS NOT NULL
    GROUP BY d.knowledge_base_id
    HAVING count(DISTINCT v.embedding_model_id) = 1
)
UPDATE knowledge_bases kb
SET embedding_model_id = avm.embedding_model_id
FROM active_vector_models avm
WHERE kb.id = avm.knowledge_base_id
  AND kb.embedding_model_id IS NULL;

-- 2. 没有有效向量证据时，使用文档记录中唯一的实际模型。
WITH document_models AS (
    SELECT knowledge_base_id, min(embedding_model_id::text)::uuid AS embedding_model_id
    FROM documents
    WHERE deleted_at IS NULL AND embedding_model_id IS NOT NULL
    GROUP BY knowledge_base_id
    HAVING count(DISTINCT embedding_model_id) = 1
)
UPDATE knowledge_bases kb
SET embedding_model_id = dm.embedding_model_id
FROM document_models dm
WHERE kb.id = dm.knowledge_base_id
  AND kb.embedding_model_id IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM documents active_doc
      JOIN document_vectors active_vec
        ON active_vec.document_id = active_doc.id
       AND active_vec.index_version = active_doc.active_index_version
       AND active_vec.status = 'ready'
      WHERE active_doc.knowledge_base_id = kb.id
        AND active_doc.deleted_at IS NULL
        AND active_doc.active_index_version IS NOT NULL
  );

-- 3. 最后使用旧知识库配置字段。
UPDATE knowledge_bases
SET embedding_model_id = default_embedding_model_id
WHERE embedding_model_id IS NULL
  AND default_embedding_model_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1
      FROM documents active_doc
      JOIN document_vectors active_vec
        ON active_vec.document_id = active_doc.id
       AND active_vec.index_version = active_doc.active_index_version
       AND active_vec.status = 'ready'
      WHERE active_doc.knowledge_base_id = knowledge_bases.id
        AND active_doc.deleted_at IS NULL
        AND active_doc.active_index_version IS NOT NULL
  )
  AND NOT EXISTS (
      SELECT 1 FROM documents model_doc
      WHERE model_doc.knowledge_base_id = knowledge_bases.id
        AND model_doc.deleted_at IS NULL
        AND model_doc.embedding_model_id IS NOT NULL
  );

-- 4. 仅对完全没有文档的空知识库使用用户默认 Embedding。
UPDATE knowledge_bases kb
SET embedding_model_id = (
    SELECT cfg.id
    FROM ai_model_configs cfg
    WHERE cfg.user_id = kb.user_id
      AND cfg.model_type = 'embedding'
      AND cfg.is_default = true
      AND cfg.enabled = true
      AND cfg.deleted_at IS NULL
    ORDER BY cfg.updated_at DESC
    LIMIT 1
)
WHERE kb.embedding_model_id IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM documents d
      WHERE d.knowledge_base_id = kb.id AND d.deleted_at IS NULL
  )
  AND EXISTS (
      SELECT 1 FROM ai_model_configs cfg
      WHERE cfg.user_id = kb.user_id
        AND cfg.model_type = 'embedding'
        AND cfg.is_default = true
        AND cfg.enabled = true
        AND cfg.deleted_at IS NULL
  );

-- Conversation 当前模型回填：AgentConfig > KnowledgeBase 旧字段 > 用户默认 Chat。
UPDATE conversations c
SET chat_model_id = ac.chat_model_id
FROM agent_configs ac
WHERE ac.knowledge_base_id = c.knowledge_base_id
  AND ac.user_id = c.user_id
  AND ac.chat_model_id IS NOT NULL
  AND c.chat_model_id IS NULL;

UPDATE conversations c
SET chat_model_id = kb.default_chat_model_id
FROM knowledge_bases kb
WHERE kb.id = c.knowledge_base_id
  AND kb.default_chat_model_id IS NOT NULL
  AND c.chat_model_id IS NULL;

UPDATE conversations c
SET chat_model_id = (
    SELECT cfg.id
    FROM ai_model_configs cfg
    WHERE cfg.user_id = c.user_id
      AND cfg.model_type = 'chat'
      AND cfg.is_default = true
      AND cfg.enabled = true
      AND cfg.deleted_at IS NULL
    ORDER BY cfg.updated_at DESC
    LIMIT 1
)
WHERE c.chat_model_id IS NULL
  AND EXISTS (
      SELECT 1 FROM ai_model_configs cfg
      WHERE cfg.user_id = c.user_id
        AND cfg.model_type = 'chat'
        AND cfg.is_default = true
        AND cfg.enabled = true
        AND cfg.deleted_at IS NULL
  );

-- Run 模型身份回填：原 AgentConfig > Assistant Message 实际模型 > Conversation 当前模型。
UPDATE agent_runs ar
SET chat_model_id = ac.chat_model_id
FROM agent_configs ac
WHERE ac.id = ar.agent_config_id
  AND ac.chat_model_id IS NOT NULL
  AND ar.chat_model_id IS NULL;

UPDATE agent_runs ar
SET chat_model_id = m.model_config_id
FROM messages m
WHERE m.agent_run_id = ar.id
  AND m.role = 'assistant'
  AND m.model_config_id IS NOT NULL
  AND ar.chat_model_id IS NULL;

UPDATE agent_runs ar
SET chat_model_id = c.chat_model_id
FROM conversations c
WHERE c.id = ar.conversation_id
  AND c.chat_model_id IS NOT NULL
  AND ar.chat_model_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_kb_embedding_model ON knowledge_bases(embedding_model_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_conversation_chat_model ON conversations(chat_model_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_agent_run_chat_model ON agent_runs(chat_model_id);

-- 为核验阶段保留可直接查询的异常清单；解决异常后再执行非空和旧列清理迁移。
CREATE OR REPLACE VIEW model_binding_migration_anomalies AS
SELECT 'knowledge_base'::text AS entity_type,
       d.knowledge_base_id AS entity_id,
       'multiple_active_embedding_models'::text AS issue_code,
       ('当前生效向量包含 ' || count(DISTINCT v.embedding_model_id)::text || ' 个 Embedding 模型')::text AS details
FROM documents d
JOIN document_vectors v
  ON v.document_id = d.id
 AND v.index_version = d.active_index_version
 AND v.status = 'ready'
WHERE d.deleted_at IS NULL AND d.active_index_version IS NOT NULL
GROUP BY d.knowledge_base_id
HAVING count(DISTINCT v.embedding_model_id) > 1
UNION ALL
SELECT 'knowledge_base', kb.id, 'embedding_model_unresolved', '无法安全确定 Embedding 模型'
FROM knowledge_bases kb
WHERE kb.deleted_at IS NULL AND kb.embedding_model_id IS NULL
UNION ALL
SELECT 'conversation', c.id, 'chat_model_unresolved', '无法安全确定 Conversation Chat 模型'
FROM conversations c
WHERE c.deleted_at IS NULL AND c.chat_model_id IS NULL
UNION ALL
SELECT 'agent_run', ar.id, 'chat_model_unresolved', '无法安全确定 Run Chat 模型'
FROM agent_runs ar
WHERE ar.chat_model_id IS NULL;

-- 新代码保证新记录必填。历史异常数据在核验完成后的清理迁移中再统一 SET NOT NULL / DROP 旧列。
