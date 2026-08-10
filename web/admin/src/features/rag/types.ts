export interface Citation {
  source_type: 'knowledge_base' | 'network';
  knowledge_base_id?: string;
  document_id?: string;
  document_title?: string;
  chunk_id?: string;
  quoted_text?: string;
  source_location?: Record<string, unknown>;
  document_updated_at?: string;
  title?: string;
  url?: string;
  site_name?: string;
  published_at?: string;
  fetched_at?: string;
}
