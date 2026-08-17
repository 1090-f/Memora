import { Alert, Stack } from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useReducer, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { capabilities } from '@/app/capabilities';
import { initialAgentRunState, reduceAgentEvent } from '@/features/agent-run/eventReducer';
import { cancelAgentRun, createAgentRun, getAgentRun } from '@/features/agent-run/api';
import { queryKeys } from '@/api/queryKeys';
import { listKnowledgeBases } from '@/features/knowledge-base/api';
import { ChatWorkspace } from '@/layouts/ChatWorkspace';
import { createConversation, listMessages } from '../api';
import { streamAgentEvents } from '../events';
import { ChatComposer } from '../components/ChatComposer';
import { MessageList } from '../components/MessageList';
import type { Citation, Message } from '../types';

const conversationStorageKey = (kbId: string) => `memora:conversation:${kbId}`;

function normalizeCitations(values: Array<Record<string, unknown>>): Citation[] {
  return values.map((value) => ({
    source_type: value.source_type === 'network' ? 'network' : 'knowledge_base',
    document_id: typeof value.document_id === 'string' ? value.document_id : undefined,
    document_title: typeof value.document_title === 'string' ? value.document_title : undefined,
    quoted_text: typeof value.quoted_text === 'string' ? value.quoted_text : undefined,
    title: typeof value.title === 'string' ? value.title : undefined,
    url: typeof value.url === 'string' ? value.url : undefined,
    site_name: typeof value.site_name === 'string' ? value.site_name : undefined,
  }));
}

export function ChatPageContent({ kbId, conversationId }: { kbId: string; conversationId?: string }) {
  const enabled = capabilities.conversation === 'available';
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [runState, dispatchRun] = useReducer(reduceAgentEvent, initialAgentRunState);
  const runStateRef = useRef(runState);
  runStateRef.current = runState;
  const [activeConversationId, setActiveConversationId] = useState(conversationId);
  const abortRef = useRef<AbortController | null>(null);
  const replayAbortRef = useRef<AbortController | null>(null);
  const hydratedRunIdRef = useRef<string | null>(null);
  const currentRunId = useRef<string | null>(null);
  const pendingConversationNavigationRef = useRef<string | null>(null);
  const [activeRunId, setActiveRunId] = useState<string | null>(null);

  useEffect(() => {
    if (conversationId && pendingConversationNavigationRef.current === conversationId) {
      pendingConversationNavigationRef.current = null;
      return;
    }
    abortRef.current?.abort();
    replayAbortRef.current?.abort();
    setActiveConversationId(conversationId);
    setMessages([]);
    currentRunId.current = null;
    setActiveRunId(null);
    hydratedRunIdRef.current = null;
    dispatchRun({ type: 'RESET_AGENT_RUN_STATE' });
  }, [kbId, conversationId]);

  const knowledgeBasesQuery = useQuery({
    queryKey: queryKeys.knowledgeBases,
    queryFn: () => listKnowledgeBases({ page: 1, page_size: 100, sort: 'updated_at_desc' }),
    enabled,
    staleTime: 30_000,
  });

  const messagesQuery = useQuery({
    queryKey: ['conversations', activeConversationId, 'messages'],
    queryFn: () => listMessages(activeConversationId as string, { page: 1, page_size: 100 }),
    enabled: enabled && Boolean(activeConversationId) && !submitting,
  });

  useEffect(() => {
    if (!messagesQuery.data) return;
    const sortedMessages = [...messagesQuery.data.items].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    setMessages(sortedMessages);
  }, [messagesQuery.data]);

  useEffect(() => {
    if (!messagesQuery.data) return;
    if (submitting) return;

    const sortedMessages = [...messagesQuery.data.items].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    const latestRunId = [...sortedMessages].reverse().find((message) => message.agent_run_id)?.agent_run_id;
    if (!latestRunId || hydratedRunIdRef.current === latestRunId) return;

    replayAbortRef.current?.abort();
    const controller = new AbortController();
    replayAbortRef.current = controller;
    hydratedRunIdRef.current = latestRunId;
    currentRunId.current = latestRunId;
    setActiveRunId(latestRunId);
    dispatchRun({ type: 'RESET_AGENT_RUN_STATE' });

    void (async () => {
      try {
        const run = await getAgentRun(latestRunId);
        if (controller.signal.aborted) return;
        dispatchRun({ type: 'HYDRATE_AGENT_RUN_STATE', run });
        const terminal = run.status === 'completed' || run.status === 'failed' || run.status === 'cancelled';
        await streamAgentEvents(`${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${latestRunId}/events`, {
          signal: controller.signal,
          afterSequence: 0,
          onEvent: dispatchRun,
          timeout: terminal ? 5000 : undefined,
        });
      } catch (error) {
        if (!controller.signal.aborted) setErrorMessage(error instanceof Error ? error.message : 'Agent 运行轨迹恢复失败');
      } finally {
        if (replayAbortRef.current === controller) replayAbortRef.current = null;
      }
    })();

    return () => controller.abort();
  }, [messagesQuery.data, submitting]);

  const getConversationId = async () => {
    if (activeConversationId) return activeConversationId;
    const conversation = await createConversation(kbId, '新会话');
    sessionStorage.setItem(conversationStorageKey(kbId), conversation.id);
    pendingConversationNavigationRef.current = conversation.id;
    setActiveConversationId(conversation.id);
    navigate(`/chat/${kbId}/${conversation.id}`, { replace: true });
    void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
    return conversation.id;
  };

  const send = async () => {
    if (!enabled || !draft.trim() || submitting) return;
    const query = draft.trim();
    setSubmitting(true);

    try {
      setErrorMessage(null);
      const id = await getConversationId();
      setDraft('');
      setMessages((current) => [...current, {
        id: crypto.randomUUID(), role: 'user', content: query, agent_run_id: null, created_at: new Date().toISOString(),
      }]);
      const response = await createAgentRun({ knowledge_base_id: kbId, conversation_id: id, query });
      replayAbortRef.current?.abort();
      currentRunId.current = response.run_id;
      setActiveRunId(response.run_id);
      hydratedRunIdRef.current = response.run_id;
      // 重置 reducer 状态，清除上一轮运行的数据（highest_sequence、answer 等）
      dispatchRun({ type: 'RESET_AGENT_RUN_STATE' });
      dispatchRun({ type: 'SET_AGENT_RUN_QUEUED' });
      const controller = new AbortController();
      abortRef.current = controller;
      await streamAgentEvents(`${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${response.run_id}/events`, {
        signal: controller.signal,
        onEvent: dispatchRun,
      });
      const completedRun = await getAgentRun(response.run_id);
      const answer = completedRun.final_result || runStateRef.current.answer;
      if (answer) {
        setMessages((current) => [...current, {
          id: crypto.randomUUID(), role: 'assistant', content: answer, agent_run_id: response.run_id,
          status: completedRun.status || 'completed', citations: normalizeCitations(runStateRef.current.citations), created_at: new Date().toISOString(),
        }]);
      }
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '智能问答请求失败');
    } finally {
      void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
      if (activeConversationId) void queryClient.invalidateQueries({ queryKey: ['conversations', activeConversationId, 'messages'] });
      setSubmitting(false);
      abortRef.current = null;
    }
  };

  const stop = () => {
    abortRef.current?.abort();
    if (currentRunId.current) void cancelAgentRun(currentRunId.current);
  };

  const composer = (
    <ChatComposer
      draft={draft}
      disabled={!enabled}
      streaming={submitting}
      knowledgeBases={knowledgeBasesQuery.data?.items ?? []}
      selectedKnowledgeBaseId={kbId}
      onDraftChange={setDraft}
      onKnowledgeBaseChange={(knowledgeBaseId) => navigate(`/chat/${knowledgeBaseId}`)}
      onSend={() => void send()}
      onStop={stop}
    />
  );
  const empty = messages.length === 0 && !submitting;
  const messageArea = (
    <Stack sx={{ flex: 1, minHeight: 0 }}>
      {!enabled && <Alert severity="info" sx={{ m: 2, mb: 0 }}>智能问答后端未启用，请检查服务配置。</Alert>}
      {errorMessage && <Alert severity="error" sx={{ m: 2, mb: 0 }}>{errorMessage}</Alert>}
      {messagesQuery.error && <Alert severity="warning" sx={{ m: 2, mb: 0 }}>历史消息加载失败，请稍后重试。</Alert>}
      <MessageList messages={messages} streamingAnswer={submitting ? runState.answer : ''} agentRunState={runState} agentRunId={activeRunId} emptyComposer={empty ? composer : undefined} onSuggestion={setDraft} />
    </Stack>
  );

  return (
    <ChatWorkspace messages={messageArea} composer={empty ? null : composer} />
  );
}

export function ChatPage() {
  const { kbId = '', conversationId } = useParams();
  return <ChatPageContent kbId={kbId} conversationId={conversationId} />;
}
