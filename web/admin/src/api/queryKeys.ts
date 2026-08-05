export const queryKeys = {
  currentUser: ['users', 'me'] as const,
  knowledgeBases: ['knowledge-bases'] as const,
  documents: (knowledgeBaseId: string) =>
    ['knowledge-bases', knowledgeBaseId, 'documents'] as const,
  conversations: (knowledgeBaseId: string) =>
    ['knowledge-bases', knowledgeBaseId, 'conversations'] as const,
  agentRuns: ['agent-runs'] as const,
  memories: ['memories'] as const,
  mcpServers: ['mcp', 'servers'] as const,
  models: ['models'] as const,
};
