export const AUTH_STORAGE_KEY = 'memora.auth';

interface StoredSession {
  access_token: string;
  expires_at: number;
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
