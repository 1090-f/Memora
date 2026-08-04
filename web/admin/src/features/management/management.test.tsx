import { render, screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import { AppProviders } from '@/app/providers';
import { server } from '@/test/server';
import { MemoryPageContent } from '@/features/memory/pages/MemoryPage';
import { McpPageContent } from '@/features/mcp/pages/McpPage';
import { SearchTestPageContent } from '@/features/search/pages/SearchTestPage';
import { ModelSettingsPageContent } from '@/features/model/pages/ModelSettingsPage';

const renderFeature = (node: React.ReactNode) => render(
  <MemoryRouter><AppProviders>{node}</AppProviders></MemoryRouter>,
);

describe('management capability states', () => {
  afterEach(() => sessionStorage.clear());

  it.each([
    ['长期记忆', <MemoryPageContent status="backend_pending" />, '更改记忆状态'],
    ['MCP 工具', <McpPageContent status="backend_pending" />, '新增 MCP 服务'],
    ['检索测试', <SearchTestPageContent status="backend_pending" kbId="kb-1" />, '执行检索测试'],
    ['模型设置', <ModelSettingsPageContent status="backend_pending" />, '新增模型'],
  ])('renders %s without HTTP calls and disables mutations', (title, node, action) => {
    let requests = 0;
    server.use(http.all('/api/v1/*', () => {
      requests += 1;
      return HttpResponse.json({});
    }));

    renderFeature(node);

    expect(screen.getByRole('heading', { name: title })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: action })).toBeDisabled();
    expect(screen.getAllByText(/后端待接入/).length).toBeGreaterThan(0);
    expect(requests).toBe(0);
  });

  it('shows request_id for an available-mode server error', async () => {
    server.use(
      http.get('/api/v1/memories', () =>
        HttpResponse.json(
          { code: 'INTERNAL_ERROR', message: '记忆服务不可用', request_id: 'req-memory-error' },
          { status: 500 },
        ),
      ),
    );

    renderFeature(<MemoryPageContent status="available" />);

    expect(await screen.findByText('记忆服务不可用')).toBeInTheDocument();
    expect(screen.getByText(/req-memory-error/)).toBeInTheDocument();
  });
});
