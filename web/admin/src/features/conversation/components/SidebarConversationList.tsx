import AddOutlined from '@mui/icons-material/AddOutlined';
import ChatBubbleOutlineOutlined from '@mui/icons-material/ChatBubbleOutlineOutlined';
import clsx from 'clsx';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { queryKeys } from '@/api/queryKeys';
import { listConversations } from '../api';

export function SidebarConversationList({ kbId, selectedId }: {
  kbId: string;
  selectedId?: string;
}) {
  const navigate = useNavigate();
  const query = useQuery({
    queryKey: queryKeys.conversations(kbId),
    queryFn: () => listConversations(kbId, { page: 1, page_size: 100 }),
    enabled: Boolean(kbId),
  });

  const createNew = () => {
    sessionStorage.removeItem(`memora:conversation:${kbId}`);
    navigate(`/chat/${kbId}`);
  };

  const items = query.data?.items ?? [];
  return (
    <section
      aria-label="最近对话"
      className="flex min-h-0 flex-1 flex-col border-t border-slate-100 px-5 pt-3"
    >
      <div className="mb-1.5 flex shrink-0 items-center justify-between px-1">
        <span className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">最近对话</span>
        <button
          type="button"
          onClick={createNew}
          aria-label="新建会话"
          title="新建会话"
          className={clsx(
            'flex h-7 w-7 items-center justify-center rounded-lg text-slate-500 transition-colors duration-200 motion-reduce:transition-none',
            'hover:bg-slate-100 hover:text-blue-600 active:scale-95',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30',
          )}
        >
          <AddOutlined className="h-4 w-4" />
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto pb-2">
        {query.isPending && <p className="px-1 py-2 text-xs text-slate-400">加载中…</p>}
        {query.error && <p className="px-1 py-2 text-xs text-slate-400">会话加载失败</p>}
        {!query.isPending && !query.error && items.length === 0 && (
          <p className="px-1 py-2 text-xs text-slate-400">暂无会话，点击 + 新建</p>
        )}
        {items.map((conversation) => {
          const active = conversation.id === selectedId;
          return (
            <button
              key={conversation.id}
              type="button"
              onClick={() => navigate(`/chat/${kbId}/${conversation.id}`)}
              className={clsx(
                'group mb-1 flex w-full items-start gap-2.5 rounded-xl px-3 py-2.5 text-left transition-colors duration-200 motion-reduce:transition-none',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30',
                active
                  ? 'bg-gradient-to-r from-blue-50 to-indigo-50'
                  : 'hover:bg-slate-50',
              )}
            >
              <ChatBubbleOutlineOutlined
                className={clsx(
                  'mt-0.5 h-4 w-4 shrink-0',
                  active ? 'text-blue-500' : 'text-slate-400 group-hover:text-slate-500',
                )}
              />
              <span className="min-w-0 flex-1">
                <span className={clsx('block truncate text-[13px] font-medium', active ? 'text-blue-700' : 'text-slate-700')}>
                  {conversation.title}
                </span>
                <span className="mt-0.5 block text-[11px] text-slate-400">
                  {new Date(conversation.updated_at || conversation.created_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}
