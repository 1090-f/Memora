export interface SearchTestRequest {
  query: string
  mode?: 'keyword' | 'semantic' | 'hybrid'
  document_ids?: string[]
  top_k?: number
}

export interface SearchResult {
  document_id: string
  document_title: string
  chunk_id: string
  content: string
  directory_id: string | null
  source_location: {
    page?: number
    section?: string
  }
  keyword_score?: number
  vector_score?: number
  rrf_rank?: number
  reranker_score?: number
  final_rank: number
  document_updated_at: string
}

export interface SearchTestResponse {
  query: string
  keyword_results: SearchResult[]
  vector_results: SearchResult[]
  rrf_results: SearchResult[]
  reranked_results: SearchResult[]
  final_results: SearchResult[]
  timing: {
    keyword_ms: number
    vector_ms: number
    rrf_ms: number
    reranker_ms: number
    total_ms: number
  }
}

export interface SearchRequest {
  query: string
  mode?: 'keyword' | 'semantic' | 'hybrid'
  document_ids?: string[]
  top_k?: number
}

export interface SearchResponse {
  query: string
  mode: string
  knowledge_status: 'sufficient' | 'insufficient' | 'ambiguous'
  results: SearchResult[]
  elapsed_ms: number
}
