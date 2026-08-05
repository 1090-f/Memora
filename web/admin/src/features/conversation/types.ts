export interface Conversation {
  id: string;
  knowledge_base_id: string;
  title: string;
  created_at: string;
  updated_at?: string;
}

export interface Citation {
  source_type: 'knowledge_base' | 'network';
  document_id?: string;
  document_title?: string;
  quoted_text?: string;
  title?: string;
  url?: string;
  site_name?: string;
}

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  agent_run_id: string | null;
  status?: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled';
  citations?: Citation[];
  created_at: string;
}

export interface QuestionResponse {
  run_id: string;
  user_message_id: string;
  status: 'queued';
  events_url: string;
}
