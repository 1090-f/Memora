import { apiRequest } from '@/api/client';
import type { ChangePasswordInput, UpdateCurrentUserInput, User } from './types';

export const getCurrentUser = () =>
  apiRequest<User>({ url: '/users/me', method: 'GET' });

export const updateCurrentUser = (input: UpdateCurrentUserInput) =>
  apiRequest<User>({ url: '/users/me', method: 'PATCH', data: input });

export const changePassword = (input: ChangePasswordInput) =>
  apiRequest<{ password_changed: boolean }>({
    url: '/users/me/password',
    method: 'PATCH',
    data: input,
  });
