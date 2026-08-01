import { request } from '@/api/client'
import type {
  ModelConfig,
  CreateModelConfigRequest,
  UpdateModelConfigRequest,
  SearchConfig,
  AgentConfig,
} from './types'

// Model Config APIs
export function listModelConfigs(): Promise<ModelConfig[]> {
  return request<ModelConfig[]>('/model-configs')
}

export function getModelConfig(id: string): Promise<ModelConfig> {
  return request<ModelConfig>(`/model-configs/${id}`)
}

export function createModelConfig(data: CreateModelConfigRequest): Promise<ModelConfig> {
  return request<ModelConfig>('/model-configs', {
    method: 'POST',
    body: data,
  })
}

export function updateModelConfig(id: string, data: UpdateModelConfigRequest): Promise<ModelConfig> {
  return request<ModelConfig>(`/model-configs/${id}`, {
    method: 'PATCH',
    body: data,
  })
}

export function deleteModelConfig(id: string): Promise<void> {
  return request<void>(`/model-configs/${id}`, {
    method: 'DELETE',
  })
}

// Search Config APIs
export function getSearchConfig(kbId: string): Promise<SearchConfig> {
  return request<SearchConfig>(`/knowledge-bases/${kbId}/search-config`)
}

export function updateSearchConfig(kbId: string, data: Partial<SearchConfig>): Promise<SearchConfig> {
  return request<SearchConfig>(`/knowledge-bases/${kbId}/search-config`, {
    method: 'PUT',
    body: data,
  })
}

// Agent Config APIs
export function getAgentConfig(kbId: string): Promise<AgentConfig> {
  return request<AgentConfig>(`/knowledge-bases/${kbId}/agent-config`)
}

export function updateAgentConfig(kbId: string, data: Partial<AgentConfig>): Promise<AgentConfig> {
  return request<AgentConfig>(`/knowledge-bases/${kbId}/agent-config`, {
    method: 'PUT',
    body: data,
  })
}
