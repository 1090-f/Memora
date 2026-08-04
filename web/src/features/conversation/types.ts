export interface Conversation {
  id: string
  knowledge_base_id: string
  title: string
  created_at: string
}

export interface CreateConversationRequest {
  title?: string
}

export type Citation = KnowledgeBaseCitation | NetworkCitation

export interface KnowledgeBaseCitation {
  source_type: 'knowledge_base'
  document_id: string
  document_title: string
  chunk_id: string
  quoted_text: string
  knowledge_base_id: string
  source_location: {
    section?: string
    page?: number
  }
  document_updated_at: string
}

export interface NetworkCitation {
  source_type: 'network'
  title: string
  url: string
  site_name: string
  published_at: string | null
  fetched_at: string
}

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  agent_run_id: string | null
  status?: 'streaming' | 'completed' | 'failed' | 'cancelled'
  citations?: Citation[]
  created_at: string
}

export interface QuestionAccepted {
  run_id: string
  user_message_id: string
  status: 'queued'
  events_url: string
}

export interface QuestionInput {
  query: string
  document_id?: string
}

export const conversationKeys = {
  all: ['conversations'] as const,
  list: (kbId: string) => ['conversations', 'list', kbId] as const,
  detail: (id: string) => ['conversations', 'detail', id] as const,
  messages: (conversationId: string) => ['messages', conversationId] as const,
}
