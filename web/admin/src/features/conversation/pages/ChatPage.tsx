import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useReducer, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { capabilities } from '@/app/capabilities';
import { AgentRunPanel } from '@/features/agent-run/components/AgentRunPanel';
import {
  initialAgentRunState,
  reduceAgentEvent,
  type ResetAction,
} from '@/features/agent-run/eventReducer';
import {
  cancelAgentRun,
  createAgentRun,
  getAgentRun,
  listAgentRuns,
  retryAgentRun,
} from '@/features/agent-run/api';
import { queryKeys } from '@/api/queryKeys';
import { ChatWorkspace } from '@/layouts/ChatWorkspace';
import {
  createConversation,
  deleteConversation,
  listConversations,
  listMessages,
  updateConversation,
} from '../api';
import { streamAgentEvents } from '../events';
import { ChatComposer } from '../components/ChatComposer';
import { ConversationSidebar } from '../components/ConversationSidebar';
import { MessageList } from '../components/MessageList';
import { groupConsecutiveAssistantMessages } from '../utils/groupMessages';
import type { Message } from '../types';

export function ChatPageContent({
  kbId,
  conversationId,
}: {
  kbId: string;
  conversationId?: string;
}) {
  const enabled = capabilities.conversation === 'available';
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [runState, dispatchRun] = useReducer(
    reduceAgentEvent,
    initialAgentRunState,
  );
  const runStateRef = useRef(runState);
  runStateRef.current = runState;
  const abortRef = useRef<AbortController | null>(null);
  const currentRunId = useRef<string | null>(null);
  const runConversationRef = useRef<string | null>(null);
  const visitedConversations = useRef<Set<string>>(new Set());
  const [shouldScrollToBottom, setShouldScrollToBottom] = useState(false);

  // 会话列表
  const conversationsQuery = useQuery({
    queryKey: queryKeys.conversations(kbId),
    queryFn: () => listConversations(kbId, { page: 1, page_size: 100 }),
    enabled,
  });

  // 当前会话信息
  const currentConversation = useMemo(
    () => conversationsQuery.data?.items?.find(c => c.id === conversationId),
    [conversationsQuery.data, conversationId],
  );

  // 切换会话时加载历史消息
  useEffect(() => {
    if (!conversationId || !enabled) {
      setMessages([]);
      setSubmitting(false);
      setShouldScrollToBottom(false);
      return;
    }

    abortRef.current?.abort();
    setSubmitting(false);
    dispatchRun({ type: 'RESET_AGENT_RUN_STATE' } as ResetAction);
    currentRunId.current = null;
    runConversationRef.current = null;
    abortRef.current = null;

    let cancelled = false;
    listAgentRuns({ knowledge_base_id: kbId, page: 1, page_size: 100 })
      .then(result => {
        if (cancelled || result.items.length === 0) return;
        const activeRun = result.items.find(
          run =>
            run.conversation_id === conversationId &&
            (run.status === 'queued' || run.status === 'running'),
        );
        if (!activeRun || cancelled) return;
        currentRunId.current = activeRun.id;
        runConversationRef.current = conversationId;
        setSubmitting(true);
        const controller = new AbortController();
        abortRef.current = controller;
        void streamAgentEvents(
          `${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${activeRun.id}/events`,
          {
            signal: controller.signal,
            onEvent: dispatchRun,
            timeout: 120_000,
          },
        )
          .catch(error => {
            if (!cancelled) console.warn('恢复 Agent 运行流失败', error);
          })
          .finally(() => {
            if (!cancelled && runConversationRef.current === conversationId) {
              setSubmitting(false);
              runConversationRef.current = null;
              abortRef.current = null;
            }
          });
      })
      .catch(() => {
        // 查询运行状态失败时保持当前会话为空闲状态
      });

    // 检查是否为首次访问该会话
    const isFirstVisit = !visitedConversations.current.has(conversationId);

    listMessages(conversationId, { page: 1, page_size: 100 })
      .then(result => {
        // 按时间排序后端消息（升序），然后分组合并连续助手消息为版本历史
        const rawMessages = result.items || [];
        const sortedMessages = [...rawMessages].sort(
          (a, b) =>
            new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
        );
        const groupedMessages =
          groupConsecutiveAssistantMessages(sortedMessages);
        setMessages(groupedMessages);

        // 仅在首次访问时滚动到底部
        if (isFirstVisit && groupedMessages.length > 0) {
          visitedConversations.current.add(conversationId);
          setShouldScrollToBottom(true);
          // 重置滚动标志，避免后续消息更新时误触发
          setTimeout(() => setShouldScrollToBottom(false), 100);
        }
      })
      .catch(() => {
        setMessages([]);
        setShouldScrollToBottom(false);
      });

    return () => {
      cancelled = true;
      if (runConversationRef.current === conversationId) {
        abortRef.current?.abort();
        abortRef.current = null;
        runConversationRef.current = null;
      }
    };
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
    runConversationRef.current = conversationId;
    try {
      setErrorMessage(null);
      setDraft('');
      setMessages(current => [
        ...current,
        {
          id: crypto.randomUUID(),
          role: 'user',
          content: query,
          agent_run_id: null,
          created_at: new Date().toISOString(),
        },
      ]);

      // 如果当前会话还是默认标题 "新会话"，用第一个问题更新会话名
      if (currentConversation?.title === '新会话') {
        try {
          await updateConversation(conversationId, query);
          void queryClient.invalidateQueries({
            queryKey: queryKeys.conversations(kbId),
          });
        } catch {
          // 标题更新失败不影响问答流程
        }
      }

      const response = await createAgentRun({
        knowledge_base_id: kbId,
        conversation_id: conversationId,
        query,
      });
      currentRunId.current = response.run_id;
      dispatchRun({ type: 'RESET_AGENT_RUN_STATE' } as ResetAction);
      const controller = new AbortController();
      abortRef.current = controller;

      // SSE 流式推送：添加超时保护，即使 SSE 异常结束也不阻塞后续答案获取
      try {
        await streamAgentEvents(
          `${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${response.run_id}/events`,
          {
            signal: controller.signal,
            onEvent: dispatchRun,
            timeout: 120_000, // 2 分钟超时保护
          },
        );
      } catch (e) {
        console.warn('SSE 流异常结束，将通过 API 获取最终结果', e);
      }

      // SSE 结束后（无论是否成功），总是尝试从 API 获取答案
      const completedRun = await activeRunQuery.refetch();
      const run = completedRun.data;
      const answer =
        run?.final_result ||
        runStateRef.current.answer ||
        (run?.status === 'failed' ? run.error_message : '');
      if (answer) {
        setMessages(current => [
          ...current,
          {
            id: crypto.randomUUID(),
            role: 'assistant',
            content: answer,
            agent_run_id: response.run_id,
            status: run?.status || 'completed',
            created_at: new Date().toISOString(),
          },
        ]);
      }
    } catch (error) {
      setErrorMessage(
        error instanceof Error ? error.message : '智能问答请求失败',
      );
    } finally {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.conversations(kbId),
      });
      if (runConversationRef.current === conversationId) {
        setSubmitting(false);
        runConversationRef.current = null;
        abortRef.current = null;
      }
    }
  };

  /**
   * handleRetry 重试指定 agent_run 的消息。
   * 方案：调用 retry API 创建新运行 → SSE 流式接收 → 结束后从后端重新加载消息列表。
   * 因为后端会持久化所有消息记录，重新加载时分组函数 groupConsecutiveAssistantMessages
   * 会自动将连续的助手消息合并为版本历史，无需前端临时维护 versions。
   */
  const handleRetry = async (agentRunId: string, _query: string) => {
    if (!enabled || submitting || !conversationId) return;
    setSubmitting(true);

    // 保存目标消息索引用于失败时恢复
    const targetIndex = messages.findIndex(m => m.agent_run_id === agentRunId);

    try {
      setErrorMessage(null);

      // 1. 调用 retry API 获取新的运行 ID
      const { new_run_id } = await retryAgentRun(agentRunId);
      currentRunId.current = new_run_id;

      // 2. 立即将目标消息标记为"正在重新生成"
      if (targetIndex >= 0) {
        setMessages(current =>
          current.map((m, i) =>
            i === targetIndex ? { ...m, status: 'running' as const } : m,
          ),
        );
      }

      // 3. 重置 Agent 运行状态，准备接收新运行的事件
      dispatchRun({ type: 'RESET_AGENT_RUN_STATE' } as ResetAction);
      const controller = new AbortController();
      abortRef.current = controller;

      // 4. SSE 流式订阅新运行的生命周期事件
      try {
        await streamAgentEvents(
          `${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${new_run_id}/events`,
          {
            signal: controller.signal,
            onEvent: dispatchRun,
            timeout: 120_000,
          },
        );
      } catch (e) {
        console.warn('重试 SSE 流异常结束，将通过 API 获取最终结果', e);
      }

      // 5. SSE 结束后从后端重新加载完整消息列表，分组函数自动合并版本历史
      const result = await listMessages(conversationId, {
        page: 1,
        page_size: 100,
      });
      const rawMessages = result.items || [];
      const sortedMessages = [...rawMessages].sort(
        (a, b) =>
          new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
      );
      const groupedMessages = groupConsecutiveAssistantMessages(sortedMessages);
      setMessages(groupedMessages);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '重试运行失败');
      // 重新加载消息列表以恢复到一致状态
      if (conversationId) {
        const result = await listMessages(conversationId, {
          page: 1,
          page_size: 100,
        }).catch(() => null);
        if (result?.items) {
          const sortedMessages = [...result.items].sort(
            (a, b) =>
              new Date(a.created_at).getTime() -
              new Date(b.created_at).getTime(),
          );
          setMessages(groupConsecutiveAssistantMessages(sortedMessages));
        }
      }
    } finally {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.conversations(kbId),
      });
      setSubmitting(false);
      abortRef.current = null;
    }
  };

  /**
   * handleSwitchVersion 切换消息的版本显示。
   * current_version_index >= 0 时显示对应历史版本内容；
   * -1 时显示 content（最新版本）。
   */
  const handleSwitchVersion = (messageId: string, newIndex: number) => {
    setMessages(current =>
      current.map(m =>
        m.id === messageId ? { ...m, current_version_index: newIndex } : m,
      ),
    );
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
    const existingNew = conversationsQuery.data?.items?.find(
      c => c.title === '新会话',
    );
    if (existingNew) {
      navigate(`/chat/${kbId}/${existingNew.id}`);
      return;
    }
    try {
      const conversation = await createConversation(kbId, '新会话');
      void queryClient.invalidateQueries({
        queryKey: queryKeys.conversations(kbId),
      });
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
      void queryClient.invalidateQueries({
        queryKey: queryKeys.conversations(kbId),
      });
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
      onDelete={id => setDeleteTarget(id)}
    />
  );

  const messageArea = (
    <>
      {!enabled && (
        <Alert severity='info' sx={{ m: 2, mb: 0 }}>
          智能问答后端未启用，请检查服务配置。
        </Alert>
      )}
      {errorMessage && (
        <Alert severity='error' sx={{ m: 2, mb: 0 }}>
          {errorMessage}
        </Alert>
      )}
      <MessageList
        messages={messages}
        streamingAnswer={submitting ? runState.answer : ''}
        onSuggestion={setDraft}
        scrollToBottom={shouldScrollToBottom}
        onRetry={handleRetry}
        onSwitchVersion={handleSwitchVersion}
      />
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
      <ChatWorkspace
        sidebar={sidebar}
        messages={messageArea}
        composer={composer}
        agentPanel={<AgentRunPanel state={runState} />}
      />
      <Dialog open={!!deleteTarget} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>确认删除</DialogTitle>
        <DialogContent>
          <DialogContentText>
            确定要删除此会话吗？删除后不可恢复。
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)}>取消</Button>
          <Button onClick={confirmDelete} color='error' variant='contained'>
            删除
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}

export function ChatPage() {
  const { kbId = '', conversationId } = useParams();
  return <ChatPageContent kbId={kbId} conversationId={conversationId} />;
}
