export type MemoryType = 'preference' | 'project' | 'decision' | 'goal' | 'fact' | 'progress';
export type MemoryScopeType = 'user' | 'knowledge_base';
export type MemoryStatus = 'active' | 'inactive' | 'deleted';
export interface Memory {
  id: string;
  memory_type: MemoryType;
  scope_type: MemoryScopeType;
  scope_id: string | null;
  content: string;
  summary: string;
  importance: number;
  source_conversation_id: string;
  source_message_id: string;
  status: MemoryStatus;
  created_at: string;
  updated_at: string;
  last_accessed_at: string | null;
}
