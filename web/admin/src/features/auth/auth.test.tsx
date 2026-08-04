import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import App from '@/App';
import { AUTH_STORAGE_KEY } from './session';
import { server } from '@/test/server';

const user = {
  id: 'user-1',
  username: 'admin',
  nickname: '管理员',
  email: 'admin@example.com',
  avatar_url: null,
  bio: null,
};

const authenticatedSession = () => {
  sessionStorage.setItem(
    AUTH_STORAGE_KEY,
    JSON.stringify({
      access_token: 'existing-token',
      expires_at: Date.now() + 60_000,
      user,
    }),
  );
};

const renderAt = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  );

describe('authentication flow', () => {
  afterEach(() => sessionStorage.clear());

  it('persists a successful login and returns to the protected destination', async () => {
    server.use(
      http.post('/api/v1/auth/login', async ({ request }) => {
        expect(await request.json()).toEqual({ account: 'admin', password: 'valid-password' });
        return HttpResponse.json({
          code: 'OK',
          message: 'success',
          data: {
            access_token: 'new-token',
            token_type: 'Bearer',
            expires_in: 3600,
            user,
          },
          request_id: 'req-login',
        });
      }),
    );
    const browser = userEvent.setup();
    renderAt('/login?redirect=%2Fmemories');

    await browser.type(screen.getByLabelText('账号'), 'admin');
    await browser.type(screen.getByLabelText('密码'), 'valid-password');
    await browser.click(screen.getByRole('button', { name: '登录' }));

    expect(
      await screen.findByRole('heading', { name: '长期记忆', level: 1 }),
    ).toBeInTheDocument();
    expect(JSON.parse(sessionStorage.getItem(AUTH_STORAGE_KEY) || '{}')).toMatchObject({
      access_token: 'new-token',
      user,
    });
  });

  it('shows a safe login error and request ID', async () => {
    server.use(
      http.post('/api/v1/auth/login', () =>
        HttpResponse.json(
          { code: 'UNAUTHORIZED', message: '账号或密码错误', request_id: 'req-denied' },
          { status: 401 },
        ),
      ),
    );
    const browser = userEvent.setup();
    renderAt('/login');

    await browser.type(screen.getByLabelText('账号'), 'admin');
    await browser.type(screen.getByLabelText('密码'), 'wrong-password');
    await browser.click(screen.getByRole('button', { name: '登录' }));

    expect(await screen.findByText('账号或密码错误')).toBeInTheDocument();
    expect(screen.getByText(/req-denied/)).toBeInTheDocument();
  });

  it('clears the local session even when logout transport fails', async () => {
    authenticatedSession();
    server.use(http.post('/api/v1/auth/logout', () => HttpResponse.error()));
    const browser = userEvent.setup();
    renderAt('/memories');

    await browser.click(await screen.findByRole('button', { name: '退出登录' }));

    expect(
      await screen.findByRole('heading', { name: '登录 Memora' }),
    ).toBeInTheDocument();
    expect(sessionStorage.getItem(AUTH_STORAGE_KEY)).toBeNull();
  });
});
