import { render, screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import { AppProviders } from '@/app/providers';
import { server } from '@/test/server';
import { KnowledgeBaseListContent } from '@/features/knowledge-base/pages/KnowledgeBaseListPage';
import { DocumentWorkspaceContent } from './pages/DocumentWorkspacePage';

const renderFeature = (node: React.ReactNode) =>
  render(
    <MemoryRouter>
      <AppProviders>{node}</AppProviders>
    </MemoryRouter>,
  );

describe('knowledge and document capabilities', () => {
  let requestCount = 0;

  afterEach(() => {
    requestCount = 0;
    sessionStorage.clear();
  });

  it('renders the knowledge shell without requests or enabled mutations while pending', () => {
    server.use(http.all('/api/v1/*', () => {
      requestCount += 1;
      return HttpResponse.json({});
    }));

    renderFeature(<KnowledgeBaseListContent status="backend_pending" />);

    expect(screen.getByRole('heading', { name: '我的知识库' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '新建知识库' })).toBeDisabled();
    expect(screen.getByText(/后端尚未提供知识库接口/)).toBeInTheDocument();
    expect(requestCount).toBe(0);
  });

  it('consumes a Memora knowledge-base envelope when enabled', async () => {
    server.use(
      http.get('/api/v1/knowledge-bases', () =>
        HttpResponse.json({
          code: 'OK',
          message: 'success',
          data: {
            items: [{
              id: 'kb-1', name: '产品知识', icon: 'book', description: '产品资料',
              document_count: 8, agent_enabled: true, network_enabled: false,
              updated_at: '2026-08-04T10:00:00Z', created_at: '2026-08-01T10:00:00Z',
            }],
            page: 1,
            page_size: 20,
            total: 1,
          },
          request_id: 'req-kb',
        }),
      ),
    );

    renderFeature(<KnowledgeBaseListContent status="available" />);

    expect(await screen.findByRole('heading', { name: '产品知识' })).toBeInTheDocument();
    expect(screen.getByText('8 篇文档')).toBeInTheDocument();
  });

  it('renders a read-only document workspace and disables imports while pending', () => {
    server.use(http.all('/api/v1/*', () => {
      requestCount += 1;
      return HttpResponse.json({});
    }));

    renderFeature(
      <DocumentWorkspaceContent status="backend_pending" kbId="kb-1" documentId="doc-1" />,
    );

    expect(screen.getByRole('heading', { name: '文档工作区' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '导入文档' })).toBeDisabled();
    expect(screen.getByText(/只读浏览将在文档接口可用后自动启用/)).toBeInTheDocument();
    expect(requestCount).toBe(0);
  });
});
