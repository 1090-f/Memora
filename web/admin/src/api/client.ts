import axios, { type AxiosRequestConfig } from 'axios';
import { readStoredSession } from '@/features/auth/session';
import { AppError } from './errors';
import type { ApiEnvelope } from './types';

type UnauthorizedHandler = (() => void) | undefined;

let unauthorizedHandler: UnauthorizedHandler;

const service = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 120_000,
  validateStatus: () => true,
});

const isEnvelope = (value: unknown): value is ApiEnvelope<unknown> => {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<ApiEnvelope<unknown>>;
  return (
    typeof candidate.code === 'string' &&
    typeof candidate.message === 'string' &&
    typeof candidate.request_id === 'string'
  );
};

export const setUnauthorizedHandler = (handler: UnauthorizedHandler) => {
  unauthorizedHandler = handler;
};

export async function apiRequest<T = unknown>(
  config: AxiosRequestConfig,
): Promise<T> {
  const session = readStoredSession();

  try {
    const response = await service.request<ApiEnvelope<T>>({
      method: 'GET',
      ...config,
      headers: {
        ...config.headers,
        ...(session ? { Authorization: `Bearer ${session.access_token}` } : {}),
      },
    });

    const body: unknown = response.data;
    if (response.status === 401) unauthorizedHandler?.();

    if (isEnvelope(body)) {
      if (response.status >= 200 && response.status < 300 && body.code === 'OK') {
        return body.data as T;
      }
      throw new AppError(
        body.code,
        body.message,
        response.status,
        body.details,
        body.request_id,
      );
    }

    throw new AppError(
      'HTTP_ERROR',
      response.status >= 500 ? '服务暂时不可用' : '请求失败',
      response.status,
      undefined,
      '',
    );
  } catch (error) {
    if (error instanceof AppError) throw error;
    throw new AppError('NETWORK_ERROR', '网络连接失败', 0, undefined, '');
  }
}
