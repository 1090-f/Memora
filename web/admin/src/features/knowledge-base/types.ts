export interface PageResult<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
}

export interface KnowledgeBase {
  id: string;
  name: string;
  icon: string;
  description: string | null;
  document_count: number;
  agent_enabled: boolean;
  network_enabled: boolean;
  updated_at: string;
  created_at: string;
}

export interface KnowledgeBaseInput {
  name: string;
  description?: string;
  icon?: string;
  default_language?: string;
  qa_enabled?: boolean;
  agent_enabled?: boolean;
  network_enabled?: boolean;
  default_chat_model_id?: string;
  default_embedding_model_id?: string;
  default_reranker_model_id?: string;
}

export interface KnowledgeBaseListParams {
  page?: number;
  page_size?: number;
  keyword?: string;
  sort?: string;
}
