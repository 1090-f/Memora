/**
 * @deprecated This file is deprecated and will be removed in a future version.
 * Do not import from this file. Use 'src/request' instead.
 */

import request from './request';
import {
  CheckModelData,
  CreateModelData,
  GetModelNameData,
  ModelListItem,
  UpdateKnowledgeBaseData,
  UpdateModelData,
} from './type';

export type * from './type';

// =============================================》knowledge base

export const updateKnowledgeBase = (
  data: Partial<UpdateKnowledgeBaseData>,
): Promise<void> =>
  request({ url: 'api/v1/knowledge_base/detail', method: 'put', data });

// =============================================》file

export const uploadFile = (
  data: FormData,
  config?: {
    onUploadProgress?: (event: { progress: number }) => void;
    abortSignal?: AbortSignal;
  },
): Promise<{ key: string; filename: string }> =>
  request({
    url: '/api/v1/file/upload',
    method: 'post',
    data,
    onUploadProgress: config?.onUploadProgress
      ? progressEvent => {
          const progress = Math.round(
            (progressEvent.loaded * 100) / (progressEvent.total || 1),
          );
          config.onUploadProgress?.({ progress });
        }
      : undefined,
    signal: config?.abortSignal,
    headers: { 'Content-Type': 'multipart/form-data' },
  });

// =============================================》model
export const getModelNameList = (
  data: GetModelNameData,
): Promise<{ models: { model: string }[]; error: string }> =>
  request({ url: 'api/v1/model/provider/supported', method: 'post', data });

export const testModel = (data: CheckModelData): Promise<{ error: string }> =>
  request({ url: 'api/v1/model/check', method: 'post', data });

export const getModelList = (): Promise<ModelListItem[]> =>
  request({ url: 'api/v1/model/list', method: 'get' });

export const createModel = (data: CreateModelData): Promise<{ id: string }> =>
  request({ url: 'api/v1/model', method: 'post', data });

export const deleteModel = (params: { id: string }): Promise<void> =>
  request({ url: 'api/v1/model', method: 'delete', params });

export const updateModel = (data: UpdateModelData): Promise<void> =>
  request({ url: 'api/v1/model', method: 'put', data });
