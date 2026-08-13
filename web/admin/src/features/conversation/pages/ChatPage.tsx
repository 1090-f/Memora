import { Alert } from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useReducer, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { capabilities } from '@/app/capabilities';
import { AgentRunPanel } from '@/features/agent-run/components/AgentRunPanel';
import { initialAgentRunState, reduceAgentEvent, type ResetAction } from '@/features/agent-run/eventReducer';
import { cancelAgentRun, createAgentRun, getAgentRun } from '@/features/agent-run/api';
import { queryKeys } from '@/api/queryKeys';
import { ChatWorkspace } from '@/layouts/ChatWorkspace';
import { createConversation, listConversations, listMessages } from '../api';
import { streamAgentEvents } from '../events';
import { ChatComposer } from '../components/ChatComposer';
import { ConversationSidebar } from '../components/ConversationSidebar';
import { MessageList } from '../components/MessageList';
import type { Message } from '../types';

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
  const abortRef = useRef<AbortController | null>(null);
  const currentRunId = useRef<string | null>(null);

  // 会话列表
  const conversationsQuery = useQuery({
    queryKey: queryKeys.conversations(kbId),
    queryFn: () => listConversations(kbId, { page: 1, page_size: 100 }),
    enabled,
  });

  // 切换会话时加载历史消息
  useEffect(() => {
    if (!conversationId || !enabled) {
      setMessages([]);
      return;
    }
    listMessages(conversationId, { page: 1, page_size: 100 }).then((result) => {
      setMessages(result.items || []);
    }).catch(() => {
      setMessages([]);
    });
  }, [conversationId, kbId, enabled]);

  // Agent 运行详情查询（用于 SSE 结束后获取最终结果）
  const activeRunQuery = useQuery({
    queryKey: ['agent-run', currentRunId.current],
    queryFn: () => {
      const runId = currentRunId.current;
      if (!runId) throw new Error('No active agent run');
      return getAgentRun(runId);
    },
    enabled: false,
  });

  const send = async () => {
    if (!enabled || !draft.trim() || submitting || !conversationId) return;
    const query = draft.trim();
    setSubmitting(true);
    try {
      setErrorMessage(null);
      setDraft('');
      setMessages((current) => [...current, {
        id: crypto.randomUUID(), role: 'user', content: query, agent_run_id: null, created_at: new Date().toISOString(),
      }]);
      const response = await createAgentRun({ knowledge_base_id: kbId, conversation_id: conversationId, query });
      currentRunId.current = response.run_id;
      dispatchRun({ type: 'RESET_AGENT_RUN_STATE' } as ResetAction);
      const controller = new AbortController();
      abortRef.current = controller;
      await streamAgentEvents(`${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${response.run_id}/events`, {
        signal: controller.signal,
        onEvent: dispatchRun,
      });
      const completedRun = await activeRunQuery.refetch();
      const answer = completedRun.data?.final_result || runStateRef.current.answer;
      if (answer) {
        setMessages((current) => [...current, {
          id: crypto.randomUUID(), role: 'assistant', content: answer, agent_run_id: response.run_id,
          status: completedRun.data?.status || 'completed', created_at: new Date().toISOString(),
        }]);
      }
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '智能问答请求失败');
    } finally {
      void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
      setSubmitting(false);
      abortRef.current = null;
    }
  };

  const stop = () => {
    abortRef.current?.abort();
    if (currentRunId.current) void cancelAgentRun(currentRunId.current);
  };

  const handleSelectConversation = (id: string) => {
    navigate(`/chat/${kbId}/${id}`);
  };

  const handleCreateConversation = async () => {
    try {
      const conversation = await createConversation(kbId, '新会话');
      void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
      navigate(`/chat/${kbId}/${conversation.id}`);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '创建会话失败');
    }
  };

  const sidebar = (
    <ConversationSidebar
      conversations={conversationsQuery.data?.items || []}
      selectedId={conversationId}
      disabled={!enabled || submitting}
      onSelect={handleSelectConversation}
      onCreate={handleCreateConversation}
    />
  );

  const messageArea = (
    <>
      {!enabled && <Alert severity="info" sx={{ m: 2, mb: 0 }}>智能问答后端未启用，请检查服务配置。</Alert>}
      {errorMessage && <Alert severity="error" sx={{ m: 2, mb: 0 }}>{errorMessage}</Alert>}
      <MessageList messages={messages} streamingAnswer={submitting ? runState.answer : ''} onSuggestion={setDraft} />
    </>
  );

  const composer = conversationId ? (
    <ChatComposer
      draft={draft}
      disabled={!enabled || submitting}
      streaming={submitting}
      onDraftChange={setDraft}
      onSend={() => void send()}
      onStop={stop}
    />
  ) : null;

  return <ChatWorkspace sidebar={sidebar} messages={messageArea} composer={composer} agentPanel={<AgentRunPanel state={runState} />} />;
}

export function ChatPage() {
  const { kbId = '', conversationId } = useParams();
  return <ChatPageContent kbId={kbId} conversationId={conversationId} />;
}