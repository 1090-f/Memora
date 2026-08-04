import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import App from '@/App';
import { AUTH_STORAGE_KEY } from '@/features/auth/session';
import { server } from '@/test/server';

const currentUser = {
  id: 'user-1',
  username: 'admin',
  nickname: '管理员',
  email: 'admin@example.com',
  avatar_url: null,
  bio: '负责知识库维护',
};

const renderProfile = () => {
  sessionStorage.setItem(
    AUTH_STORAGE_KEY,
    JSON.stringify({
      access_token: 'profile-token',
      expires_at: Date.now() + 60_000,
      user: currentUser,
    }),
  );
  return render(
    <MemoryRouter initialEntries={['/settings/profile']}>
      <App />
    </MemoryRouter>,
  );
};

describe('current user profile', () => {
  afterEach(() => sessionStorage.clear());

  it('loads and updates the current user with Memora field names', async () => {
    let updateBody: unknown;
    server.use(
      http.get('/api/v1/users/me', () =>
        HttpResponse.json({
          code: 'OK',
          message: 'success',
          data: currentUser,
          request_id: 'req-me',
        }),
      ),
      http.patch('/api/v1/users/me', async ({ request }) => {
        updateBody = await request.json();
        return HttpResponse.json({
          code: 'OK',
          message: 'success',
          data: { ...currentUser, nickname: '知识管理员' },
          request_id: 'req-update',
        });
      }),
    );
    const browser = userEvent.setup();
    renderProfile();

    const nickname = await screen.findByLabelText('昵称');
    expect(nickname).toHaveValue('管理员');
    await browser.clear(nickname);
    await browser.type(nickname, '知识管理员');
    await browser.click(screen.getByRole('button', { name: '保存资料' }));

    expect(await screen.findByText('资料已更新')).toBeInTheDocument();
    expect(updateBody).toEqual({
      nickname: '知识管理员',
      email: 'admin@example.com',
      bio: '负责知识库维护',
    });
  });

  it('rejects a new password shorter than 12 characters without a request', async () => {
    let passwordRequests = 0;
    server.use(
      http.get('/api/v1/users/me', () =>
        HttpResponse.json({
          code: 'OK',
          message: 'success',
          data: currentUser,
          request_id: 'req-me',
        }),
      ),
      http.patch('/api/v1/users/me/password', () => {
        passwordRequests += 1;
        return HttpResponse.json({
          code: 'OK',
          message: 'success',
          data: { password_changed: true },
          request_id: 'req-password',
        });
      }),
    );
    const browser = userEvent.setup();
    renderProfile();

    await browser.type(await screen.findByLabelText('当前密码'), 'current-password');
    await browser.type(screen.getByLabelText('新密码'), 'too-short');
    await browser.click(screen.getByRole('button', { name: '修改密码' }));

    expect(await screen.findByText('新密码至少需要 12 个字符')).toBeInTheDocument();
    expect(passwordRequests).toBe(0);
  });
});
