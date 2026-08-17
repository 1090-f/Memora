export interface PageResult<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
}

export interface KnowledgeBase {
  id: string;
  name: string;
  icon: string | null;
  description: string | null;
  document_count: number;
  agent_enabled: boolean;
  network_enabled: boolean;
  updated_at: string;
  created_at: string;
}

export interface KnowledgeBaseDetail {
  id: string;
  name: string;
  description: string | null;
  icon: string | null;
  default_language: string;
  qa_enabled: boolean;
  agent_enabled: boolean;
  network_enabled: boolean;
  default_chat_model_id: string | null;
  default_embedding_model_id: string | null;
  default_reranker_model_id: string | null;
  duplicate_policy: 'skip' | 'create_new';
  created_at: string;
  updated_at: string;
}

export interface KnowledgeBaseImportTrendPoint {
  date: string;
  count: number;
}

export interface KnowledgeBaseActivity {
  id: string;
  title: string;
  description: string;
  status: string;
  occurred_at: string;
}

export interface KnowledgeBaseDashboard {
  health_score: number;
  document_total: number;
  indexed_total: number;
  processing_total: number;
  failed_total: number;
  highest_active_index_version: number;
  import_trend: KnowledgeBaseImportTrendPoint[];
  recent_activities: KnowledgeBaseActivity[];
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

export interface KnowledgeBaseUpdateInput {
  name?: string;
  description?: string;
  icon?: string;
  default_language?: string;
  qa_enabled?: boolean;
  agent_enabled?: boolean;
  network_enabled?: boolean;
  default_chat_model_id?: string;
  default_embedding_model_id?: string;
  default_reranker_model_id?: string;
  duplicate_policy?: 'skip' | 'create_new';
}

export interface KnowledgeBaseListParams {
  page?: number;
  page_size?: number;
  keyword?: string;
  sort?: string;
}

export interface SearchConfig {
  keyword_top_k: number;
  vector_top_k: number;
  min_vector_score: number;
  rrf_k: number;
  rrf_top_k: number;
  reranker_top_k: number;
  reranker_threshold: number | null;
  minimum_effective_results: number;
  reranker_model_id: string | null;
}

export interface SearchConfigUpdateInput {
  keyword_top_k?: number;
  vector_top_k?: number;
  min_vector_score?: number;
  rrf_k?: number;
  rrf_top_k?: number;
  reranker_top_k?: number;
  reranker_threshold?: number;
  minimum_effective_results?: number;
  reranker_model_id?: string;
}
