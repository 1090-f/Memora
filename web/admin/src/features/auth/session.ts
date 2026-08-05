import type { LoginResponse } from './types';
import type { User } from '@/features/user/types';

export const AUTH_STORAGE_KEY = 'memora.auth';

export interface StoredSession {
  access_token: string;
  expires_at: number;
  user?: User;
}

export const readStoredSession = (): StoredSession | null => {
  const raw = sessionStorage.getItem(AUTH_STORAGE_KEY);
  if (!raw) return null;

  try {
    const value = JSON.parse(raw) as Partial<StoredSession>;
    if (
      typeof value.access_token !== 'string' ||
      value.access_token.length === 0 ||
      typeof value.expires_at !== 'number' ||
      value.expires_at <= Date.now()
    ) {
      sessionStorage.removeItem(AUTH_STORAGE_KEY);
      return null;
    }
    return value as StoredSession;
  } catch {
    sessionStorage.removeItem(AUTH_STORAGE_KEY);
    return null;
  }
};

export const saveLoginSession = (response: LoginResponse): StoredSession => {
  const session: StoredSession = {
    access_token: response.access_token,
    expires_at: Date.now() + response.expires_in * 1000,
    user: response.user,
  };
  sessionStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(session));
  return session;
};

export const updateStoredUser = (user: User) => {
  const session = readStoredSession();
  if (!session) return;
  sessionStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify({ ...session, user }));
};

export const clearStoredSession = () => {
  sessionStorage.removeItem(AUTH_STORAGE_KEY);
};
