import { Alert, Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle } from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useReducer, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { capabilities } from '@/app/capabilities';
import { AgentRunPanel } from '@/features/agent-run/components/AgentRunPanel';
import { initialAgentRunState, reduceAgentEvent, type ResetAction } from '@/features/agent-run/eventReducer';
import { cancelAgentRun, createAgentRun, getAgentRun } from '@/features/agent-run/api';
import { queryKeys } from '@/api/queryKeys';
import { ChatWorkspace } from '@/layouts/ChatWorkspace';
import { createConversation, deleteConversation, listConversations, listMessages, updateConversation } from '../api';
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

  // 当前会话信息
  const currentConversation = useMemo(() =>
    conversationsQuery.data?.items?.find(c => c.id === conversationId),
    [conversationsQuery.data, conversationId],
  );

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

      // 如果当前会话还是默认标题 "新会话"，用第一个问题更新会话名
      if (currentConversation?.title === '新会话') {
        try {
          await updateConversation(conversationId, query);
          void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
        } catch {
          // 标题更新失败不影响问答流程
        }
      }

      const response = await createAgentRun({ knowledge_base_id: kbId, conversation_id: conversationId, query });
      currentRunId.current = response.run_id;
      dispatchRun({ type: 'RESET_AGENT_RUN_STATE' } as ResetAction);
      const controller = new AbortController();
      abortRef.current = controller;

      // SSE 流式推送：添加超时保护，即使 SSE 异常结束也不阻塞后续答案获取
      try {
        await streamAgentEvents(`${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${response.run_id}/events`, {
          signal: controller.signal,
          onEvent: dispatchRun,
          timeout: 120_000, // 2 分钟超时保护
        });
      } catch (e) {
        console.warn('SSE 流异常结束，将通过 API 获取最终结果', e);
      }

      // SSE 结束后（无论是否成功），总是尝试从 API 获取答案
      const completedRun = await activeRunQuery.refetch();
      const run = completedRun.data;
      const answer = run?.final_result
        || runStateRef.current.answer
        || (run?.status === 'failed' ? run.error_message : '');
      if (answer) {
        setMessages((current) => [...current, {
          id: crypto.randomUUID(), role: 'assistant', content: answer, agent_run_id: response.run_id,
          status: run?.status || 'completed', created_at: new Date().toISOString(),
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
    // 如果已存在标题为 "新会话" 的未使用会话，直接跳转不再新建
    const existingNew = conversationsQuery.data?.items?.find(c => c.title === '新会话');
    if (existingNew) {
      navigate(`/chat/${kbId}/${existingNew.id}`);
      return;
    }
    try {
      const conversation = await createConversation(kbId, '新会话');
      void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
      navigate(`/chat/${kbId}/${conversation.id}`);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '创建会话失败');
    }
  };

  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteConversation(deleteTarget);
      void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
      if (deleteTarget === conversationId) {
        navigate(`/chat/${kbId}`);
      }
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '删除会话失败');
    } finally {
      setDeleteTarget(null);
    }
  };

  const sidebar = (
    <ConversationSidebar
      conversations={conversationsQuery.data?.items || []}
      selectedId={conversationId}
      disabled={!enabled || submitting}
      onSelect={handleSelectConversation}
      onCreate={handleCreateConversation}
      onDelete={(id) => setDeleteTarget(id)}
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

  return (
    <>
      <ChatWorkspace sidebar={sidebar} messages={messageArea} composer={composer} agentPanel={<AgentRunPanel state={runState} />} />
      <Dialog open={!!deleteTarget} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>确认删除</DialogTitle>
        <DialogContent>
          <DialogContentText>确定要删除此会话吗？删除后不可恢复。</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)}>取消</Button>
          <Button onClick={confirmDelete} color="error" variant="contained">删除</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}

export function ChatPage() {
  const { kbId = '', conversationId } = useParams();
  return <ChatPageContent kbId={kbId} conversationId={conversationId} />;
}