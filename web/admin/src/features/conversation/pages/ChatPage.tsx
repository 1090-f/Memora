import { Alert } from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useReducer, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { capabilities } from '@/app/capabilities';
import { AgentRunPanel } from '@/features/agent-run/components/AgentRunPanel';
import { initialAgentRunState, reduceAgentEvent } from '@/features/agent-run/eventReducer';
import { cancelAgentRun, createAgentRun, getAgentRun } from '@/features/agent-run/api';
import { ChatWorkspace } from '@/layouts/ChatWorkspace';
import { streamAgentEvents } from '../events';
import { ChatComposer } from '../components/ChatComposer';
import { ConversationSidebar } from '../components/ConversationSidebar';
import { MessageList } from '../components/MessageList';
import type { Message } from '../types';

const conversationStorageKey = (kbId: string) => `memora:conversation:${kbId}`;

export function ChatPageContent({ kbId, conversationId }: { kbId: string; conversationId?: string }) {
  const enabled = capabilities.conversation === 'available';
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [runState, dispatchRun] = useReducer(reduceAgentEvent, initialAgentRunState);
  const [activeConversationId, setActiveConversationId] = useState(conversationId);
  const abortRef = useRef<AbortController | null>(null);
  const currentRunId = useRef<string | null>(null);

  // 通过 Agent 运行详情刷新最终回答，避免只依赖 SSE 连接是否正常结束。
  const activeRunQuery = useQuery({
    queryKey: ['agent-run', currentRunId.current],
    queryFn: () => {
      const runId = currentRunId.current;
      if (!runId) throw new Error('No active agent run');
      return getAgentRun(runId);
    },
    enabled: false,
  });

  const getConversationId = () => {
    if (activeConversationId) return activeConversationId;
    const stored = sessionStorage.getItem(conversationStorageKey(kbId));
    const id = stored || crypto.randomUUID();
    sessionStorage.setItem(conversationStorageKey(kbId), id);
    setActiveConversationId(id);
    navigate(`/chat/${kbId}/${id}`, { replace: true });
    return id;
  };

  const send = async () => {
    if (!enabled || !draft.trim() || submitting) return;
    const query = draft.trim();
    const id = getConversationId();
    setSubmitting(true);
    setDraft('');
    setMessages((current) => [...current, {
      id: crypto.randomUUID(), role: 'user', content: query, agent_run_id: null, created_at: new Date().toISOString(),
    }]);

    try {
      setErrorMessage(null);
      const response = await createAgentRun({ knowledge_base_id: kbId, conversation_id: id, query });
      currentRunId.current = response.run_id;
      const controller = new AbortController();
      abortRef.current = controller;
      await streamAgentEvents(`${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${response.run_id}/events`, {
        signal: controller.signal,
        afterSequence: runState.highest_sequence || undefined,
        onEvent: dispatchRun,
      });
      const completedRun = await activeRunQuery.refetch();
      const answer = completedRun.data?.final_result || runState.answer;
      if (answer) {
        setMessages((current) => [...current, {
          id: crypto.randomUUID(), role: 'assistant', content: answer, agent_run_id: response.run_id,
          status: completedRun.data?.status || 'completed', created_at: new Date().toISOString(),
        }]);
      }
      void queryClient.invalidateQueries({ queryKey: ['agent-runs', kbId] });
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '智能问答请求失败');
    } finally {
      setSubmitting(false);
      abortRef.current = null;
    }
  };

  const stop = () => {
    abortRef.current?.abort();
    if (currentRunId.current) void cancelAgentRun(currentRunId.current);
  };

  const sidebar = (
    <ConversationSidebar
      conversations={[]}
      selectedId={activeConversationId}
      disabled={!enabled || submitting}
      onSelect={(id) => navigate(`/chat/${kbId}/${id}`)}
      onCreate={() => navigate(`/chat/${kbId}`)}
    />
  );
  const messageArea = (
    <>
      {!enabled && <Alert severity="info" sx={{ m: 2, mb: 0 }}>智能问答后端未启用，请检查服务配置。</Alert>}
      {errorMessage && <Alert severity="error" sx={{ m: 2, mb: 0 }}>{errorMessage}</Alert>}
      <MessageList messages={messages} streamingAnswer={runState.answer} onSuggestion={setDraft} />
    </>
  );
  const composer = (
    <ChatComposer
      draft={draft}
      disabled={!enabled || submitting}
      streaming={submitting}
      onDraftChange={setDraft}
      onSend={() => void send()}
      onStop={stop}
    />
  );

  return <ChatWorkspace sidebar={sidebar} messages={messageArea} composer={composer} agentPanel={<AgentRunPanel state={runState} />} />;
}

export function ChatPage() {
  const { kbId = '', conversationId } = useParams();
  return <ChatPageContent kbId={kbId} conversationId={conversationId} />;
}
