import { apiRequest } from '@/api/client';
import { clearStoredSession } from './session';
import type { LoginRequest, LoginResponse } from './types';

export const login = (input: LoginRequest) =>
  apiRequest<LoginResponse>({ url: '/auth/login', method: 'POST', data: input });

export const logout = async (): Promise<void> => {
  try {
    await apiRequest<void>({ url: '/auth/logout', method: 'POST' });
  } finally {
    clearStoredSession();
  }
};
