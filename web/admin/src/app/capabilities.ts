export type CapabilityKey =
  | 'auth'
  | 'user'
  | 'knowledgeBase'
  | 'document'
  | 'conversation'
  | 'agentRun'
  | 'memory'
  | 'mcp'
  | 'search'
  | 'model';

export type CapabilityStatus = 'available' | 'backend_pending';

export const capabilities: Record<CapabilityKey, CapabilityStatus> = {
  auth: 'available',
  user: 'available',
  knowledgeBase: 'available',
  document: 'available',
  conversation: 'backend_pending',
  agentRun: 'backend_pending',
  memory: 'available',
  mcp: 'available',
  search: 'available',
  model: 'available',
};
