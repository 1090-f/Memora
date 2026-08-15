export const queryKeys = {
  currentUser: ['users', 'me'] as const,
  knowledgeBases: ['knowledge-bases'] as const,
  // 资源级查询键保持统一，便于上传、重试和删除后精确刷新关联缓存。
  knowledgeBase: (knowledgeBaseId: string) =>
    ['knowledge-bases', knowledgeBaseId] as const,
  directories: (knowledgeBaseId: string) =>
    ['knowledge-bases', knowledgeBaseId, 'directories'] as const,
  documents: (knowledgeBaseId: string) =>
    ['knowledge-bases', knowledgeBaseId, 'documents'] as const,
  document: (documentId: string) => ['documents', documentId] as const,
  documentProcessing: (documentId: string) =>
    ['documents', documentId, 'processing'] as const,
  documentContent: (documentId: string) =>
    ['documents', documentId, 'content'] as const,
  documentIndexVersions: (documentId: string) =>
    ['documents', documentId, 'index-versions'] as const,
  importTasks: (knowledgeBaseId: string) =>
    ['knowledge-bases', knowledgeBaseId, 'import-tasks'] as const,
  conversations: (knowledgeBaseId: string) =>
    ['knowledge-bases', knowledgeBaseId, 'conversations'] as const,
  conversationMessages: (conversationId: string) =>
    ['conversations', conversationId, 'messages'] as const,
  agentRuns: ['agent-runs'] as const,
  memories: ['memories'] as const,
  mcpServers: ['mcp', 'servers'] as const,
  models: ['models'] as const,
};
