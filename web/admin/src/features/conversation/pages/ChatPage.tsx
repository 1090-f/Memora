import KeyboardArrowDownOutlined from '@mui/icons-material/KeyboardArrowDownOutlined';
import SmartToyOutlined from '@mui/icons-material/SmartToyOutlined';
import { Alert, Box, Chip, Stack, Typography } from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useReducer, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { capabilities } from '@/app/capabilities';
import { AgentRunPanel } from '@/features/agent-run/components/AgentRunPanel';
import { initialAgentRunState, reduceAgentEvent, type ResetAction } from '@/features/agent-run/eventReducer';
import { cancelAgentRun, createAgentRun, getAgentRun } from '@/features/agent-run/api';
import { queryKeys } from '@/api/queryKeys';
import { getKnowledgeBase } from '@/features/knowledge-base/api';
import { ChatWorkspace } from '@/layouts/ChatWorkspace';
import { createConversation, getConversation, listConversations, listMessages } from '../api';
import { streamAgentEvents } from '../events';
import { ChatComposer } from '../components/ChatComposer';
import { ConversationSidebar } from '../components/ConversationSidebar';
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
  const currentRunId = useRef<string | null>(null);

  useEffect(() => {
    setActiveConversationId(conversationId);
    setMessages([]);
    currentRunId.current = null;
    dispatchRun({ type: 'RESET_AGENT_RUN_STATE' } as ResetAction);
  }, [conversationId]);

  const knowledgeBaseQuery = useQuery({
    queryKey: queryKeys.knowledgeBase(kbId),
    queryFn: () => getKnowledgeBase(kbId),
    enabled: enabled && Boolean(kbId),
  });

  // 查询会话列表
  const conversationsQuery = useQuery({
    queryKey: queryKeys.conversations(kbId),
    queryFn: () => listConversations(kbId, { page: 1, page_size: 100 }),
    enabled: enabled,
  });

  const messagesQuery = useQuery({
    queryKey: ['conversations', activeConversationId, 'messages'],
    queryFn: () => listMessages(activeConversationId as string, { page: 1, page_size: 100 }),
    enabled: enabled && Boolean(activeConversationId) && !submitting,
  });

  useEffect(() => {
    if (messagesQuery.data) {
      setMessages([...messagesQuery.data.items].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()));
    }
  }, [messagesQuery.data]);

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

  const getConversationId = async () => {
    if (activeConversationId) return activeConversationId;
    const stored = sessionStorage.getItem(conversationStorageKey(kbId));
    if (stored) {
      try {
        await getConversation(stored);
        setActiveConversationId(stored);
        navigate(`/chat/${kbId}/${stored}`, { replace: true });
        return stored;
      } catch {
        sessionStorage.removeItem(conversationStorageKey(kbId));
      }
    }
    const conversation = await createConversation(kbId, '新会话');
    sessionStorage.setItem(conversationStorageKey(kbId), conversation.id);
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
      currentRunId.current = response.run_id;
      // 重置 reducer 状态，清除上一轮运行的数据（highest_sequence、answer 等）
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
          status: completedRun.data?.status || 'completed', citations: normalizeCitations(runStateRef.current.citations), created_at: new Date().toISOString(),
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

  const createNewConversation = async () => {
    if (!enabled || submitting) return;
    try {
      setErrorMessage(null);
      const conversation = await createConversation(kbId, '新会话');
      sessionStorage.setItem(conversationStorageKey(kbId), conversation.id);
      setActiveConversationId(conversation.id);
      navigate(`/chat/${kbId}/${conversation.id}`);
      void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '创建会话失败');
    }
  };

  const sidebar = (
    <ConversationSidebar
      conversations={conversationsQuery.data?.items || []}
      selectedId={activeConversationId}
      disabled={!enabled || submitting}
      onSelect={(id) => navigate(`/chat/${kbId}/${id}`)}
      onCreate={() => void createNewConversation()}
    />
  );
  const messageArea = (
    <Stack sx={{ flex: 1, minHeight: 0 }}>
      <Stack direction="row" alignItems="center" spacing={1.2} sx={{ minHeight: 56, px: 2, borderBottom: '1px solid #e6e9ef', bgcolor: '#fff' }}>
        <Box sx={{ width: 30, height: 30, borderRadius: 1.5, display: 'grid', placeItems: 'center', color: '#fff', background: 'linear-gradient(145deg,#6377f6,#6f4ee7)' }}><SmartToyOutlined sx={{ fontSize: 18 }} /></Box>
        <Typography sx={{ color: '#26324d', fontSize: 14, fontWeight: 650 }}>{knowledgeBaseQuery.data?.name || '知识库'}</Typography>
        <KeyboardArrowDownOutlined sx={{ color: '#77849b', fontSize: 18 }} />
        <Chip size="small" label={knowledgeBaseQuery.data?.agent_enabled ? '●  Agent 已启用' : 'Agent 未启用'} sx={{ ml: 1, bgcolor: knowledgeBaseQuery.data?.agent_enabled ? '#e8f7ec' : '#f1f3f7', color: knowledgeBaseQuery.data?.agent_enabled ? '#24954c' : '#7d8799', fontWeight: 600 }} />
      </Stack>
      {!enabled && <Alert severity="info" sx={{ m: 2, mb: 0 }}>智能问答后端未启用，请检查服务配置。</Alert>}
      {errorMessage && <Alert severity="error" sx={{ m: 2, mb: 0 }}>{errorMessage}</Alert>}
      {messagesQuery.error && <Alert severity="warning" sx={{ m: 2, mb: 0 }}>历史消息加载失败，请稍后重试。</Alert>}
      <MessageList messages={messages} streamingAnswer={submitting ? runState.answer : ''} onSuggestion={setDraft} />
    </Stack>
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

  return (
    <Stack spacing={2} sx={{ width: '100%', maxWidth: 1500, mx: 'auto' }}>
      <Stack spacing={0.7}>
        <Stack direction="row" spacing={1} alignItems="center">
          <Typography component={Link} to="/knowledge-bases" sx={{ color: '#748098', fontSize: 13, textDecoration: 'none' }}>知识库</Typography>
          <Typography sx={{ color: '#a0a8b7', fontSize: 13 }}>/</Typography>
          <Typography sx={{ color: '#526079', fontSize: 13 }}>{knowledgeBaseQuery.data?.name || '智能问答'}</Typography>
        </Stack>
        <Typography component="h2" sx={{ color: '#111c3a', fontSize: { xs: 25, md: 28 }, fontWeight: 700, lineHeight: 1.2 }}>智能问答工作台</Typography>
        <Typography sx={{ color: '#66728c', fontSize: 14 }}>基于所选知识库进行问答、检索与 Agent 协作。</Typography>
      </Stack>
      <ChatWorkspace sidebar={sidebar} messages={messageArea} composer={composer} agentPanel={<AgentRunPanel state={runState} />} />
    </Stack>
  );
}

export function ChatPage() {
  const { kbId = '', conversationId } = useParams();
  return <ChatPageContent kbId={kbId} conversationId={conversationId} />;
}
