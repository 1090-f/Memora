import AccountCircleOutlined from '@mui/icons-material/AccountCircleOutlined';
import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import BuildOutlined from '@mui/icons-material/BuildOutlined';
import KeyboardArrowRightOutlined from '@mui/icons-material/KeyboardArrowRightOutlined';
import MemoryOutlined from '@mui/icons-material/MemoryOutlined';
import MenuBookOutlined from '@mui/icons-material/MenuBookOutlined';
import MenuOpenOutlined from '@mui/icons-material/MenuOpenOutlined';
import MenuOutlined from '@mui/icons-material/MenuOutlined';
import NotificationsNoneOutlined from '@mui/icons-material/NotificationsNoneOutlined';
import QuestionAnswerOutlined from '@mui/icons-material/QuestionAnswerOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import TimelineOutlined from '@mui/icons-material/TimelineOutlined';
import type { SvgIconProps } from '@mui/material';
import clsx from 'clsx';
import { useEffect, useRef, useState, type ComponentType } from 'react';
import { NavLink, Outlet, useLocation } from 'react-router-dom';
import { useAppSelector } from '@/store';

const navigation: Array<{
  label: string;
  path: string;
  icon: ComponentType<SvgIconProps>;
}> = [
  { label: '知识库', path: '/knowledge-bases', icon: MenuBookOutlined },
  { label: '智能问答', path: '/chat', icon: QuestionAnswerOutlined },
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

  const title =
    pageTitles[location.pathname] ||
    (location.pathname.startsWith('/chat') ? '智能问答' : undefined) ||
    (location.pathname.includes('/docs') ? '文档工作区' : undefined) ||
    (location.pathname.includes('/search-test') ? '检索测试' : undefined) ||
    (location.pathname.includes('/settings') ? '知识库设置' : 'Memora');

  return (
    <div className="min-h-screen bg-[#fbfcff]">
      <aside
        aria-label="侧边栏导航"
        className={clsx(
          sidebarBase,
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

        <nav aria-label="主导航" className="mt-2 flex-1 overflow-y-auto px-5 pb-4">
          {navigation.map((item) => {
            const active = isPathActive(item.path);
            const Icon = item.icon;
            return (
              <NavLink
                key={item.path}
                to={item.path}
                title={collapsed ? item.label : undefined}
                aria-label={collapsed ? item.label : undefined}
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
                  {item.label}
                </span>
              </NavLink>
            );
          })}
        </nav>

        <div
          className={clsx(
            'shrink-0 p-3',
            collapsed && 'lg:p-2',
          )}
        >
          {user && (
            <div
              className={clsx(
                'flex items-center gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3',
                collapsed && 'lg:justify-center lg:px-0',
              )}
            >
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-blue-500 to-indigo-400 text-sm font-medium text-white shadow-[0_6px_14px_rgba(86,105,255,0.25)]">
                {(user.nickname || user.email || 'A').trim().charAt(0).toUpperCase()}
              </span>
              <div className={clsx('min-w-0 flex-1', collapsed && 'lg:hidden')}>
                <p className="truncate text-sm font-semibold text-slate-900">
                  {user.nickname}
                </p>
                <p className="truncate text-xs text-slate-400">{user.email}</p>
              </div>
              <KeyboardArrowRightOutlined
                className={clsx('h-5 w-5 text-slate-500', collapsed && 'lg:hidden')}
              />
            </div>
          )}
        </div>
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

      <header
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
          <label className="flex h-11 w-full max-w-[326px] items-center gap-3 rounded-xl border border-slate-200 bg-white px-4 text-slate-400 shadow-sm transition focus-within:border-blue-400 focus-within:ring-4 focus-within:ring-blue-500/10">
            <SearchOutlined className="h-5 w-5 shrink-0" />
            <input
              ref={searchInputRef}
              type="search"
              aria-label="搜索知识库"
              placeholder="搜索知识库..."
              className="min-w-0 flex-1 bg-transparent text-sm text-slate-800 outline-none placeholder:text-slate-400"
            />
            <span className="shrink-0 rounded-md border border-slate-200 px-1.5 py-0.5 text-xs text-slate-400 shadow-sm">
              Ctrl K
            </span>
          </label>
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
            <span className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-blue-500 to-indigo-400 text-sm font-medium text-white shadow-[0_6px_14px_rgba(86,105,255,0.25)]">
              {(user.nickname || user.email || 'A').trim().charAt(0).toUpperCase()}
            </span>
          )}
        </div>
      </header>

      <main
        className={clsx(
          'min-h-screen pt-20',
          collapsed ? 'lg:pl-20' : 'lg:pl-72',
          'transition-[padding-left] duration-200 motion-reduce:transition-none',
        )}
      >
        <div className="px-4 py-5 md:px-7 md:py-7 lg:px-12 lg:py-7">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
