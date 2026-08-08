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
  knowledgeBase: 'backend_pending',
  document: 'backend_pending',
  conversation: 'backend_pending',
  agentRun: 'backend_pending',
  memory: 'backend_pending',
  mcp: 'available',
  search: 'backend_pending',
  model: 'available',
};
