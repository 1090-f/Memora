import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import MenuBookOutlined from '@mui/icons-material/MenuBookOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import { useQuery } from '@tanstack/react-query';
import { useEffect, useRef, useState, type KeyboardEvent, type RefObject } from 'react';
import { useNavigate } from 'react-router-dom';
import { queryKeys } from '@/api/queryKeys';
import { listKnowledgeBases } from '@/features/knowledge-base/api';

interface GlobalKnowledgeBaseSearchProps {
  inputRef: RefObject<HTMLInputElement | null>;
}

export function GlobalKnowledgeBaseSearch({ inputRef }: GlobalKnowledgeBaseSearchProps) {
  const navigate = useNavigate();
  const rootRef = useRef<HTMLDivElement>(null);
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const normalizedQuery = query.trim();

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(normalizedQuery), 250);
    return () => window.clearTimeout(timer);
  }, [normalizedQuery]);

  useEffect(() => {
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', closeOnOutsideClick);
    return () => document.removeEventListener('mousedown', closeOnOutsideClick);
  }, []);

  const searchQuery = useQuery({
    queryKey: [...queryKeys.knowledgeBases, 'global-search', debouncedQuery],
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 6, keyword: debouncedQuery, sort: 'updated_at_desc' }),
    enabled: open && debouncedQuery.length > 0,
    staleTime: 30_000,
  });

  const results = searchQuery.data?.items ?? [];
  const isWaitingForDebounce = normalizedQuery !== debouncedQuery;

  useEffect(() => {
    setActiveIndex(0);
  }, [debouncedQuery, results.length]);

  const enterKnowledgeBase = (knowledgeBaseId: string) => {
    setQuery('');
    setDebouncedQuery('');
    setOpen(false);
    void navigate(`/kb/${knowledgeBaseId}/docs`);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault();
      setOpen(false);
      event.currentTarget.blur();
      return;
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setOpen(true);
      if (results.length > 0) setActiveIndex((index) => (index + 1) % results.length);
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setOpen(true);
      if (results.length > 0) setActiveIndex((index) => (index - 1 + results.length) % results.length);
      return;
    }
    if (event.key === 'Enter' && open && results[activeIndex]) {
      event.preventDefault();
      enterKnowledgeBase(results[activeIndex].id);
    }
  };

  const showDropdown = open && normalizedQuery.length > 0;
  const isLoading = isWaitingForDebounce || searchQuery.isFetching;

  return (
    <div ref={rootRef} className="relative w-full max-w-[326px]">
      <label className="flex h-11 w-full items-center gap-3 rounded-xl border border-slate-200 bg-white px-4 text-slate-400 shadow-sm transition focus-within:border-blue-400 focus-within:ring-4 focus-within:ring-blue-500/10">
        <SearchOutlined className="h-5 w-5 shrink-0" />
        <input
          ref={inputRef}
          type="search"
          value={query}
          role="combobox"
          aria-label="搜索知识库"
          aria-autocomplete="list"
          aria-expanded={showDropdown}
          aria-controls="global-knowledge-base-results"
          aria-activedescendant={showDropdown && results[activeIndex] ? `global-kb-result-${results[activeIndex].id}` : undefined}
          placeholder="搜索知识库..."
          className="min-w-0 flex-1 bg-transparent text-sm text-slate-800 outline-none placeholder:text-slate-400"
          onChange={(event) => {
            setQuery(event.target.value);
            setOpen(event.target.value.trim().length > 0);
          }}
          onFocus={() => setOpen(normalizedQuery.length > 0)}
          onKeyDown={handleKeyDown}
        />
        <span className="shrink-0 rounded-md border border-slate-200 px-1.5 py-0.5 text-xs text-slate-400 shadow-sm">
          Ctrl K
        </span>
      </label>

      {showDropdown && (
        <div id="global-knowledge-base-results" role="listbox" className="absolute left-0 right-0 top-[calc(100%+8px)] overflow-hidden rounded-2xl border border-slate-200 bg-white p-1.5 text-left shadow-[0_16px_40px_rgba(15,23,42,0.14)]">
          {isLoading && (
            <div className="flex h-20 items-center justify-center gap-2 text-sm text-slate-500">
              <span className="h-4 w-4 animate-spin rounded-full border-2 border-blue-500 border-t-transparent" />
              正在搜索…
            </div>
          )}

          {!isLoading && searchQuery.isError && (
            <div className="px-4 py-5 text-center text-sm text-red-500">搜索失败，请稍后重试</div>
          )}

          {!isLoading && !searchQuery.isError && results.length === 0 && (
            <div className="px-4 py-5 text-center text-sm text-slate-500">未找到相关知识库</div>
          )}

          {!isLoading && !searchQuery.isError && results.map((knowledgeBase, index) => (
            <button
              id={`global-kb-result-${knowledgeBase.id}`}
              key={knowledgeBase.id}
              type="button"
              role="option"
              aria-selected={index === activeIndex}
              className={`flex w-full items-center gap-3 rounded-xl px-3 py-2.5 transition ${index === activeIndex ? 'bg-blue-50 text-blue-700' : 'text-slate-700 hover:bg-slate-50'}`}
              onMouseEnter={() => setActiveIndex(index)}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => enterKnowledgeBase(knowledgeBase.id)}
            >
              <span className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${index === activeIndex ? 'bg-white text-blue-600' : 'bg-indigo-50 text-indigo-500'}`}>
                <MenuBookOutlined className="h-5 w-5" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-semibold">{knowledgeBase.name}</span>
                <span className="mt-0.5 block truncate text-xs text-slate-400">{knowledgeBase.description || '暂无描述'}</span>
              </span>
              <span className="flex shrink-0 items-center gap-1 text-xs text-slate-400">
                <DescriptionOutlined className="h-4 w-4" />
                {knowledgeBase.document_count}
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
