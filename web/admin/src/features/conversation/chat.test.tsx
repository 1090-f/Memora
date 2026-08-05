import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import { AppProviders } from '@/app/providers';
import { server } from '@/test/server';
import { ChatPageContent } from './pages/ChatPage';

const renderChat = () => render(
  <MemoryRouter>
    <AppProviders>
      <ChatPageContent status="backend_pending" kbId="kb-1" />
    </AppProviders>
  </MemoryRouter>,
);

describe('chat workspace', () => {
  afterEach(() => localStorage.clear());

  it('renders three columns and makes zero requests while pending', () => {
    let requests = 0;
    server.use(http.all('/api/v1/*', () => {
      requests += 1;
      return HttpResponse.json({});
    }));

    renderChat();

    expect(screen.getByRole('complementary', { name: '会话列表' })).toBeInTheDocument();
    expect(screen.getByRole('main', { name: '消息区' })).toBeInTheDocument();
    expect(screen.getByRole('complementary', { name: 'Agent 运行面板' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '输入问题' })).toBeDisabled();
    expect(requests).toBe(0);
  });

  it('copies a suggestion to the draft without submitting it', async () => {
    const browser = userEvent.setup();
    renderChat();

    await browser.click(screen.getByRole('button', { name: '总结当前知识库' }));

    expect(screen.getByRole('textbox', { name: '输入问题' })).toHaveValue('总结当前知识库');
    expect(screen.getByText('等待提问')).toBeInTheDocument();
  });

  it('clamps and persists the Agent panel width', () => {
    renderChat();
    const slider = screen.getByRole('slider', { name: 'Agent 面板宽度' });

    fireEvent.change(slider, { target: { value: '999' } });

    expect(slider).toHaveValue('480');
    expect(JSON.parse(localStorage.getItem('memora.layout.chat') || '{}')).toMatchObject({ agent_panel_width: 480 });
  });
});
