import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AUTH_STORAGE_KEY } from '@/features/auth/session';
import { server } from '@/test/server';
import { apiRequest, setUnauthorizedHandler } from './client';
import { AppError } from './errors';

const envelope = <T,>(data: T) => ({
  code: 'OK',
  message: 'success',
  data,
  request_id: 'req-success',
});

const setSession = () => {
  sessionStorage.setItem(
    AUTH_STORAGE_KEY,
    JSON.stringify({ access_token: 'secret-token', expires_at: Date.now() + 60_000 }),
  );
};

describe('apiRequest', () => {
  afterEach(() => {
    sessionStorage.clear();
    setUnauthorizedHandler(undefined);
  });

  it('unwraps data from a successful Memora envelope', async () => {
    server.use(
      http.get('/api/v1/example', () =>
        HttpResponse.json(envelope({ value: 'ready' })),
      ),
    );

    await expect(apiRequest<{ value: string }>({ url: '/example' })).resolves.toEqual({
      value: 'ready',
    });
  });

  it('injects Bearer authentication only when a session exists', async () => {
    const headers: Array<string | null> = [];
    server.use(
      http.get('/api/v1/auth-check', ({ request }) => {
        headers.push(request.headers.get('authorization'));
        return HttpResponse.json(envelope({ accepted: true }));
      }),
    );

    await apiRequest({ url: '/auth-check' });
    setSession();
    await apiRequest({ url: '/auth-check' });

    expect(headers).toEqual([null, 'Bearer secret-token']);
  });

  it('converts a non-OK envelope into AppError', async () => {
    server.use(
      http.get('/api/v1/rejected', () =>
        HttpResponse.json(
          {
            code: 'INVALID_ARGUMENT',
            message: '参数不正确',
            details: { field: 'account' },
            request_id: 'req-invalid',
          },
          { status: 400 },
        ),
      ),
    );

    await expect(apiRequest({ url: '/rejected' })).rejects.toMatchObject({
      code: 'INVALID_ARGUMENT',
      message: '参数不正确',
      httpStatus: 400,
      details: { field: 'account' },
      requestId: 'req-invalid',
    } satisfies Partial<AppError>);
  });

  it('calls the unauthorized handler once for a 401 response', async () => {
    const onUnauthorized = vi.fn();
    setUnauthorizedHandler(onUnauthorized);
    server.use(
      http.get('/api/v1/private', () =>
        HttpResponse.json(
          { code: 'UNAUTHORIZED', message: '请重新登录', request_id: 'req-401' },
          { status: 401 },
        ),
      ),
    );

    await expect(apiRequest({ url: '/private' })).rejects.toBeInstanceOf(AppError);
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it('normalizes a non-JSON upstream failure', async () => {
    server.use(
      http.get(
        '/api/v1/upstream',
        () => new HttpResponse('bad gateway', { status: 502 }),
      ),
    );

    await expect(apiRequest({ url: '/upstream' })).rejects.toMatchObject({
      code: 'HTTP_ERROR',
      httpStatus: 502,
      requestId: '',
    } satisfies Partial<AppError>);
  });

  it('normalizes a transport failure without leaking Axios internals', async () => {
    server.use(http.get('/api/v1/offline', () => HttpResponse.error()));

    await expect(apiRequest({ url: '/offline' })).rejects.toMatchObject({
      code: 'NETWORK_ERROR',
      httpStatus: 0,
      requestId: '',
    } satisfies Partial<AppError>);
  });
});
