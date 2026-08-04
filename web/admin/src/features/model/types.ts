export type ModelType = 'chat' | 'embedding' | 'reranker';
export interface ModelConfig {
  id: string;
  model_type: ModelType;
  provider: string;
  name: string;
  base_url: string;
  api_key_masked: string;
  timeout_seconds: number;
  retry_times: number;
  max_tokens: number | null;
  temperature: number | null;
  vector_dimension: number | null;
  supports_tool_calling: boolean;
  supports_streaming: boolean;
  is_default: boolean;
  enabled: boolean;
}
