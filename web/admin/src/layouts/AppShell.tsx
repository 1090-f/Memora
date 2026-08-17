import AccountCircleOutlined from '@mui/icons-material/AccountCircleOutlined';
import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import BuildOutlined from '@mui/icons-material/BuildOutlined';
import KeyboardArrowDownOutlined from '@mui/icons-material/KeyboardArrowDownOutlined';
import MemoryOutlined from '@mui/icons-material/MemoryOutlined';
import MenuBookOutlined from '@mui/icons-material/MenuBookOutlined';
import MenuOpenOutlined from '@mui/icons-material/MenuOpenOutlined';
import MenuOutlined from '@mui/icons-material/MenuOutlined';
import NotificationsNoneOutlined from '@mui/icons-material/NotificationsNoneOutlined';
import QuestionAnswerOutlined from '@mui/icons-material/QuestionAnswerOutlined';
import TimelineOutlined from '@mui/icons-material/TimelineOutlined';
import type { SvgIconProps } from '@mui/material';
import { useQuery } from '@tanstack/react-query';
import clsx from 'clsx';
import { useEffect, useRef, useState, type ComponentType } from 'react';
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom';
import { GlobalKnowledgeBaseSearch } from '@/components/shared/GlobalKnowledgeBaseSearch';
import { capabilities } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { SidebarConversationList } from '@/features/conversation/components/SidebarConversationList';
import { listKnowledgeBases } from '@/features/knowledge-base/api';
import { useAppSelector } from '@/store';

const navigation: Array<{
  label: string;
  path: string;
  icon: ComponentType<SvgIconProps>;
}> = [
  { label: '智能问答', path: '/chat', icon: QuestionAnswerOutlined },
  { label: '知识库', path: '/knowledge-bases', icon: MenuBookOutlined },
  { label: '运行记录', path: '/runs', icon: TimelineOutlined },
  { label: '长期记忆', path: '/memories', icon: MemoryOutlined },
  { label: 'MCP 工具', path: '/mcp', icon: BuildOutlined },
  { label: '模型设置', path: '/settings/models', icon: AutoAwesomeOutlined },
  { label: '个人资料', path: '/settings/profile', icon: AccountCircleOutlined },
];

const pageTitles: Record<string, string> = {
  '/knowledge-bases': '知识库',
  '/runs': 'Agent 运行记录',
  '/memories': '长期记忆',
  '/mcp': 'MCP 工具',
  '/settings/models': '模型设置',
  '/settings/profile': '个人资料',
};

const sidebarBase =
  'fixed inset-y-0 left-0 z-40 flex flex-col border-r border-slate-200/80 bg-white';

const navLinkBase =
  'mb-2 flex h-12 w-full items-center gap-3 rounded-2xl px-4 text-[15px] font-medium ' +
  'transition-colors duration-200 motion-reduce:transition-none ' +
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30';

export function AppShell() {
  const location = useLocation();
  const user = useAppSelector((state) => state.auth.user);
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const searchInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setDrawerOpen(false);
  }, [location.pathname]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setDrawerOpen(false);
      }
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'b') {
        const target = event.target as HTMLElement | null;
        if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA')) {
          return;
        }
        event.preventDefault();
        setCollapsed((value) => !value);
      }
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        searchInputRef.current?.focus();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const isPathActive = (path: string): boolean => {
    if (path === '/chat') {
      return location.pathname.startsWith('/chat');
    }
    if (path === '/knowledge-bases') {
      return (
        location.pathname === '/knowledge-bases' ||
        location.pathname.startsWith('/kb/')
      );
    }
    return location.pathname === path;
  };

  // 对话路由 /chat/:kbId/:conversationId? —— 会话列表挂在导航栏“最近对话”区。
  const chatSegments = location.pathname.startsWith('/chat/')
    ? location.pathname.slice('/chat/'.length).split('/')
    : [];
  const chatKbId = chatSegments[0] || undefined;
  const chatConversationId = chatSegments[1] || undefined;
  const projectKbId = location.pathname.startsWith('/kb/')
    ? location.pathname.slice('/kb/'.length).split('/')[0] || undefined
    : undefined;
  const routeKbId = chatKbId || projectKbId;
  const storedKbId = sessionStorage.getItem('memora:last-active-kb') || undefined;
  const knowledgeBasesQuery = useQuery({
    queryKey: queryKeys.knowledgeBases,
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 100, sort: 'updated_at_desc' }),
    enabled: capabilities.knowledgeBase === 'available',
    staleTime: 30_000,
  });
  const knowledgeBases = knowledgeBasesQuery.data?.items;
  const validStoredKbId = storedKbId && (!knowledgeBases || knowledgeBases.some((kb) => kb.id === storedKbId))
    ? storedKbId
    : undefined;
  const activeKbId = routeKbId || validStoredKbId || knowledgeBases?.[0]?.id;
  const isChatWorkspace = Boolean(chatKbId);

  useEffect(() => {
    if (routeKbId) sessionStorage.setItem('memora:last-active-kb', routeKbId);
  }, [routeKbId]);

  const title =
    pageTitles[location.pathname] ||
    (location.pathname.startsWith('/chat') ? '智能问答' : undefined) ||
    (location.pathname.includes('/docs') ? '文档工作区' : undefined) ||
    (location.pathname.includes('/search-test') ? '检索测试' : undefined) ||
    (location.pathname.includes('/settings') ? '知识库设置' : 'Memora');

  return (
    <div className={clsx('min-h-screen', isChatWorkspace ? 'bg-white' : 'bg-[#fbfcff]')}>
      <aside
        aria-label="侧边栏导航"
        className={clsx(
          sidebarBase,
          isChatWorkspace && 'bg-[#f3f6fa]',
          'w-72 -translate-x-full lg:translate-x-0',
          'transition-[width,transform] duration-200 motion-reduce:transition-none',
          collapsed && 'lg:w-20',
          drawerOpen && 'translate-x-0',
        )}
      >
        <div
          className={clsx(
            'flex h-20 shrink-0 items-center gap-3 px-7',
            collapsed && 'lg:justify-center lg:px-0',
          )}
        >
          <span
            className={clsx(
              'flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-indigo-500 text-white shadow-[0_8px_20px_rgba(67,94,255,0.22)]',
              collapsed && 'lg:hidden',
            )}
          >
            <MemoryOutlined className="h-6 w-6" />
          </span>
          <span
            className={clsx(
              'truncate text-xl font-bold tracking-tight text-[#111c3a]',
              collapsed && 'lg:hidden',
            )}
          >
            Memora
          </span>
          <button
            type="button"
            onClick={() => setCollapsed((value) => !value)}
            aria-label={collapsed ? '展开侧边栏' : '收起侧边栏'}
            title={collapsed ? '展开侧边栏' : '收起侧边栏 (Ctrl+B)'}
            className={clsx(
              'hidden lg:flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-slate-500',
              'transition-colors duration-200 motion-reduce:transition-none',
              'hover:bg-slate-100 hover:text-slate-900 active:scale-95',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30',
              collapsed ? 'lg:mx-auto' : 'lg:ml-auto',
            )}
          >
            {collapsed ? (
              <MenuOutlined className="h-5 w-5" />
            ) : (
              <MenuOpenOutlined className="h-5 w-5" />
            )}
          </button>
        </div>

        <div className="mt-2 flex min-h-0 flex-1 flex-col overflow-hidden">
          <nav aria-label="主导航" className="shrink-0 px-5 pb-2">
            {navigation.map((item) => {
              const active = item.path === '/chat' && chatKbId
                ? !chatConversationId
                : isPathActive(item.path);
              const Icon = item.icon;
              const itemPath = item.path === '/chat'
                ? activeKbId ? `/chat/${activeKbId}` : '/knowledge-bases'
                : item.path;
              const itemLabel = item.path === '/chat' ? '新问答' : item.label;
              return (
                <NavLink
                  key={item.path}
                  to={itemPath}
                  title={collapsed ? itemLabel : undefined}
                  aria-label={collapsed ? itemLabel : undefined}
                  className={clsx(
                    navLinkBase,
                    collapsed && 'lg:mx-auto lg:w-12 lg:justify-center lg:px-0',
                    active
                      ? 'bg-gradient-to-r from-blue-50 to-indigo-50 text-blue-600'
                      : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900',
                  )}
                >
                  <Icon className="h-5 w-5 shrink-0" />
                  <span className={clsx('truncate', collapsed && 'lg:hidden')}>
                    {itemLabel}
                  </span>
                </NavLink>
              );
            })}
          </nav>
          {capabilities.conversation === 'available' && activeKbId && (
            <div className={clsx('flex min-h-0 flex-1 flex-col', collapsed && 'lg:hidden')}>
              <SidebarConversationList kbId={activeKbId} selectedId={chatConversationId} />
            </div>
          )}
        </div>

        {user && (
          <Link
            to="/settings/profile"
            aria-label="进入个人资料"
            className={clsx(
              'mx-3 mb-3 flex shrink-0 items-center gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-slate-900 transition hover:border-blue-200 hover:bg-blue-50/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30',
              collapsed && 'lg:mx-2 lg:justify-center lg:px-0',
            )}
          >
            <span className="relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-blue-500 to-indigo-400 text-sm font-medium text-white shadow-[0_6px_14px_rgba(86,105,255,0.25)]">
              {(user.nickname || user.email || 'A').trim().charAt(0).toUpperCase()}
              {user.avatar_url && <img src={user.avatar_url} alt={`${user.nickname} 的头像`} className="absolute inset-0 h-full w-full object-cover" onError={(event) => { event.currentTarget.style.display = 'none'; }} />}
            </span>
            <span className={clsx('min-w-0 flex-1', collapsed && 'lg:hidden')}>
              <span className="block truncate text-sm font-semibold">{user.nickname}</span>
              <span className="block truncate text-xs text-slate-400">{user.email}</span>
            </span>
            <KeyboardArrowDownOutlined className={clsx('h-5 w-5 text-slate-500', collapsed && 'lg:hidden')} />
          </Link>
        )}
      </aside>

      <div
        aria-hidden="true"
        onClick={() => setDrawerOpen(false)}
        className={clsx(
          'fixed inset-0 z-30 bg-slate-950/40 lg:hidden',
          'transition-opacity duration-200 motion-reduce:transition-none',
          drawerOpen ? 'opacity-100' : 'pointer-events-none opacity-0',
        )}
      />

      {isChatWorkspace && (
        <button
          type="button"
          onClick={() => setDrawerOpen(true)}
          aria-label="打开导航菜单"
          className="fixed left-3 top-3 z-20 flex h-10 w-10 items-center justify-center rounded-xl border border-slate-200 bg-white/90 text-slate-600 shadow-sm backdrop-blur transition hover:bg-slate-50 lg:hidden"
        >
          <MenuOutlined className="h-5 w-5" />
        </button>
      )}

      {!isChatWorkspace && <header
        className={clsx(
          'fixed inset-x-0 top-0 z-30 flex h-20 items-center gap-3 border-b border-slate-200/70 bg-white/90 px-4 backdrop-blur-xl md:px-7',
          'transition-[left] duration-200 motion-reduce:transition-none',
          collapsed ? 'lg:left-20' : 'lg:left-72',
        )}
      >
        <button
          type="button"
          onClick={() => setDrawerOpen((value) => !value)}
          aria-expanded={drawerOpen}
          aria-label={drawerOpen ? '关闭导航菜单' : '打开导航菜单'}
          className="rounded-xl p-2 text-slate-600 transition-colors duration-200 motion-reduce:transition-none hover:bg-slate-100 hover:text-slate-900 active:scale-95 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30 lg:hidden"
        >
          <MenuOutlined className="h-5 w-5" />
        </button>
        <h1 className="truncate text-base font-semibold tracking-tight text-slate-900 md:text-lg lg:hidden">
          {title}
        </h1>
        <div className="hidden min-w-0 flex-1 items-center justify-center lg:flex">
          <GlobalKnowledgeBaseSearch inputRef={searchInputRef} />
        </div>
        <div className="ml-auto hidden items-center gap-5 lg:flex">
          <button
            type="button"
            aria-label="通知"
            className="relative flex h-10 w-10 items-center justify-center rounded-xl text-slate-700 transition hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30"
          >
            <NotificationsNoneOutlined className="h-6 w-6" />
            <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-blue-500 ring-2 ring-white" />
          </button>
          {user && (
            <Link
              to="/settings/profile"
              aria-label="进入个人资料"
              title="个人资料"
              className="relative flex h-10 w-10 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-blue-500 to-indigo-400 text-sm font-medium text-white shadow-[0_6px_14px_rgba(86,105,255,0.25)] transition duration-200 hover:scale-105 hover:shadow-[0_8px_20px_rgba(86,105,255,0.35)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40 focus-visible:ring-offset-2 active:scale-95"
            >
              {(user.nickname || user.email || 'A').trim().charAt(0).toUpperCase()}
              {user.avatar_url && <img src={user.avatar_url} alt={`${user.nickname} 的头像`} className="absolute inset-0 h-full w-full object-cover" onError={(event) => { event.currentTarget.style.display = 'none'; }} />}
            </Link>
          )}
        </div>
      </header>}

      <main
        className={clsx(
          'min-h-screen',
          isChatWorkspace ? 'h-screen overflow-hidden' : 'pt-20',
          collapsed ? 'lg:pl-20' : 'lg:pl-72',
          'transition-[padding-left] duration-200 motion-reduce:transition-none',
        )}
      >
        <div className={clsx(isChatWorkspace ? 'h-full p-0' : 'px-4 py-5 md:px-7 md:py-7 lg:px-12 lg:py-7')}>
          <Outlet />
        </div>
      </main>
    </div>
  );
}
