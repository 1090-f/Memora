import type { Citation } from '@/features/rag/types';

export type SearchMode = 'keyword' | 'vector' | 'hybrid';
export interface SearchInput { query: string; mode: SearchMode; document_ids?: string[]; top_k?: number }
export interface SearchResult {
  document_id: string;
  document_title: string;
  chunk_id: string;
  content: string;
  directory_id?: string;
  source_location?: Record<string, unknown>;
  score?: number;
  keyword_score?: number;
  match_level?: 'exact' | 'strong' | 'weak';
  matched_terms?: string[];
  coverage?: number;
  recall_stage?: 'exact' | 'strong' | 'weak_fallback';
  low_confidence?: boolean;
  vector_score?: number;
  keyword_rank?: number;
  vector_rank?: number;
  rrf_rank?: number;
  reranker_score?: number;
  final_rank?: number;
  index_version: number;
  document_updated_at?: string;
  citation: Citation;
}
export interface SearchResponse {
  query: string;
  mode: SearchMode;
  items: SearchResult[];
  rewritten_query?: string;
  knowledge_status: 'sufficient' | 'insufficient';
  elapsed_ms?: number;
}
export interface SearchTestResponse {
  query: string;
  keyword_results: SearchResult[];
  vector_results: SearchResult[];
  rrf_results: SearchResult[];
  reranked_results: SearchResult[];
  final_results: SearchResult[];
  knowledge_status: 'sufficient' | 'insufficient';
  timing: Partial<Record<'keyword_ms' | 'vector_ms' | 'rrf_ms' | 'reranker_ms' | 'total_ms', number>>;
}
