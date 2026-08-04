export type ModelType = 'chat' | 'embedding' | 'reranker'

export interface ModelConfig {
  id: string
  model_type: ModelType
  provider: string
  name: string
  base_url: string
  api_key_masked?: string
  timeout_seconds: number
  retry_times: number
  max_tokens: number | null
  temperature: number | null
  vector_dimension: number | null
  supports_tool_calling: boolean
  supports_streaming: boolean
  is_default: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface CreateModelConfigRequest {
  model_type: ModelType
  provider: string
  name: string
  base_url: string
  api_key?: string
  timeout_seconds?: number
  retry_times?: number
  max_tokens?: number | null
  temperature?: number | null
  vector_dimension?: number | null
  supports_tool_calling?: boolean
  supports_streaming?: boolean
  is_default?: boolean
  enabled?: boolean
}

export interface UpdateModelConfigRequest {
  provider?: string
  name?: string
  base_url?: string
  api_key?: string
  timeout_seconds?: number
  retry_times?: number
  max_tokens?: number | null
  temperature?: number | null
  vector_dimension?: number | null
  supports_tool_calling?: boolean
  supports_streaming?: boolean
  is_default?: boolean
  enabled?: boolean
}

export interface SearchConfig {
  keyword_top_k: number
  vector_top_k: number
  rrf_k: number
  rrf_top_k: number
  reranker_top_k: number
  reranker_threshold: number
  minimum_effective_results: number
  reranker_model_id: string | null
}

export interface AgentConfig {
  id: string
  name: string
  system_prompt: string
  chat_model_id: string
  max_react_rounds: number
  max_plan_steps: number
  max_replans: number
  reviewer_runs: number
  max_tool_calls: number
  max_document_read_tokens: number
  max_tool_result_bytes: number
  max_run_seconds: number
  network_enabled: boolean
  memory_enabled: boolean
  memory_top_k: number
  show_execution_status: boolean
  status: 'active' | 'inactive'
}

export const modelConfigKeys = {
  all: ['model-configs'] as const,
  list: ['model-configs', 'list'] as const,
  detail: (id: string) => ['model-configs', 'detail', id] as const,
}

export const searchConfigKeys = {
  detail: (kbId: string) => ['search-config', kbId] as const,
}

export const agentConfigKeys = {
  detail: (kbId: string) => ['agent-config', kbId] as const,
}
