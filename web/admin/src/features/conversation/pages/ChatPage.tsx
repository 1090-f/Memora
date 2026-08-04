import { Alert } from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useReducer, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { AgentRunPanel } from '@/features/agent-run/components/AgentRunPanel';
import { initialAgentRunState, reduceAgentEvent } from '@/features/agent-run/eventReducer';
import { cancelAgentRun } from '@/features/agent-run/api';
import { ChatWorkspace } from '@/layouts/ChatWorkspace';
import { createConversation, listConversations, listMessages, submitQuestion } from '../api';
import { streamAgentEvents } from '../events';
import { ChatComposer } from '../components/ChatComposer';
import { ConversationSidebar } from '../components/ConversationSidebar';
import { MessageList } from '../components/MessageList';

export function ChatPageContent({ status, kbId, conversationId }: {
  status: CapabilityStatus;
  kbId: string;
  conversationId?: string;
}) {
  const enabled = status === 'available';
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [runState, dispatchRun] = useReducer(reduceAgentEvent, initialAgentRunState);
  const createPromise = useRef<Promise<string> | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const currentRunId = useRef<string | null>(null);

  const conversationsQuery = useQuery({
    queryKey: queryKeys.conversations(kbId),
    queryFn: () => listConversations(kbId, { page: 1, page_size: 50 }),
    enabled,
  });
  const messagesQuery = useQuery({
    queryKey: ['conversations', conversationId, 'messages'],
    queryFn: () => listMessages(conversationId as string, { page: 1, page_size: 50 }),
    enabled: enabled && Boolean(conversationId),
  });

  const ensureConversation = async () => {
    if (conversationId) return conversationId;
    if (!createPromise.current) {
      createPromise.current = createConversation(kbId, draft.slice(0, 40) || '新会话')
        .then((conversation) => {
          navigate(`/chat/${kbId}/${conversation.id}`, { replace: true });
          void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
          return conversation.id;
        })
        .finally(() => { createPromise.current = null; });
    }
    return createPromise.current;
  };

  const send = async () => {
    if (!enabled || !draft.trim() || submitting) return;
    const query = draft.trim();
    setSubmitting(true);
    try {
      const id = await ensureConversation();
      const response = await submitQuestion(id, query);
      setDraft('');
      currentRunId.current = response.run_id;
      const controller = new AbortController();
      abortRef.current = controller;
      await streamAgentEvents(response.events_url, {
        signal: controller.signal,
        afterSequence: runState.highest_sequence || undefined,
        onEvent: dispatchRun,
      });
      await queryClient.invalidateQueries({ queryKey: ['conversations', id, 'messages'] });
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
      conversations={conversationsQuery.data?.items ?? []}
      selectedId={conversationId}
      disabled={!enabled || submitting}
      onSelect={(id) => navigate(`/chat/${kbId}/${id}`)}
      onCreate={() => navigate(`/chat/${kbId}`)}
    />
  );
  const messages = (
    <>
      {!enabled && <Alert severity="info" sx={{ m: 2, mb: 0 }}>会话后端待接入；工作区不会发起请求或开启事件流。</Alert>}
      <MessageList messages={messagesQuery.data?.items ?? []} streamingAnswer={runState.answer} onSuggestion={setDraft} />
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

  return <ChatWorkspace sidebar={sidebar} messages={messages} composer={composer} agentPanel={<AgentRunPanel state={runState} />} />;
}

export function ChatPage() {
  const { kbId = '', conversationId } = useParams();
  return <ChatPageContent status={capabilities.conversation} kbId={kbId} conversationId={conversationId} />;
}
