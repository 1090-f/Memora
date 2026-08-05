export type SearchMode = 'keyword' | 'semantic' | 'hybrid';
export interface SearchInput { query: string; mode: SearchMode; document_ids?: string[]; top_k?: number }
export interface SearchResult {
  document_id: string;
  document_title: string;
  chunk_id: string;
  content: string;
  directory_id: string;
  source_location: Record<string, unknown>;
  keyword_score: number | null;
  vector_score: number | null;
  rrf_rank: number | null;
  reranker_score: number | null;
  final_rank: number;
  document_updated_at: string;
}
export interface SearchTestResponse {
  query: string;
  keyword_results: SearchResult[];
  vector_results: SearchResult[];
  rrf_results: SearchResult[];
  reranked_results: SearchResult[];
  final_results: SearchResult[];
  timing: { keyword_ms: number; vector_ms: number; rrf_ms: number; reranker_ms: number; total_ms: number };
}
