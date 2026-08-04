import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { http, HttpResponse } from 'msw';
import { MemoryRouter, useLocation } from 'react-router-dom';
import App from '@/App';
import { server } from '@/test/server';

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{location.pathname + location.search}</output>;
}

const renderAt = (path: string) =>
  render(
    <MemoryRouter initialEntries={[path]}>
      <App />
      <LocationProbe />
    </MemoryRouter>,
  );

describe('Memora application routes', () => {
  afterEach(() => {
    sessionStorage.clear();
  });

  it('redirects an anonymous protected route to login', async () => {
    renderAt('/memories');

    expect(
      await screen.findByRole('heading', { name: '登录 Memora' }),
    ).toBeInTheDocument();
    expect(screen.getByTestId('location')).toHaveTextContent(
      '/login?redirect=%2Fmemories',
    );
  });

  it('renders a protected page when a session exists', async () => {
    sessionStorage.setItem(
      'memora.auth',
      JSON.stringify({ access_token: 'test-token', expires_at: Date.now() + 60_000 }),
    );

    renderAt('/memories');

    expect(
      await screen.findByRole('heading', { name: '长期记忆', level: 1 }),
    ).toBeInTheDocument();
  });

  it.each([
    ['/knowledge-bases', '知识库'],
    ['/kb/kb-1/docs', '文档工作区'],
    ['/chat/kb-1', '智能问答'],
    ['/runs', 'Agent 运行记录'],
    ['/memories', '长期记忆'],
    ['/mcp', 'MCP 工具'],
    ['/kb/kb-1/search-test', '检索测试'],
    ['/kb/kb-1/settings', '知识库设置'],
    ['/settings/profile', '个人资料'],
    ['/settings/models', '模型设置'],
  ])('renders %s with the page title %s', async (path, title) => {
    server.use(
      http.get('/api/v1/users/me', () =>
        HttpResponse.json({
          code: 'OK',
          message: 'success',
          data: {
            id: 'user-1',
            username: 'admin',
            nickname: '管理员',
            email: 'admin@example.com',
            avatar_url: null,
            bio: null,
          },
          request_id: 'req-router',
        }),
      ),
    );
    sessionStorage.setItem(
      'memora.auth',
      JSON.stringify({ access_token: 'test-token', expires_at: Date.now() + 60_000 }),
    );

    renderAt(path);

    expect(
      await screen.findByRole('heading', { name: title, level: 1 }),
    ).toBeInTheDocument();
  });
});
