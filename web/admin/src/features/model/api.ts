import { apiRequest } from '@/api/client';
import type { PageResult } from '@/features/knowledge-base/types';
import type { ModelConfig } from './types';
export const listModelConfigs = (params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<ModelConfig>>({ url: '/model-configs', params });
export const createModelConfig = (input: Omit<ModelConfig, 'id' | 'api_key_masked'> & { api_key: string }) =>
  apiRequest<ModelConfig>({ url: '/model-configs', method: 'POST', data: input });
export const updateModelConfig = (id: string, input: Partial<ModelConfig> & { api_key?: string }) =>
  apiRequest<ModelConfig>({ url: `/model-configs/${id}`, method: 'PATCH', data: input });
