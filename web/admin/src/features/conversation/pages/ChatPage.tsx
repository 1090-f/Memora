import { Alert, Stack } from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useReducer, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { capabilities } from '@/app/capabilities';
import { initialAgentRunState, reduceAgentEvent } from '@/features/agent-run/eventReducer';
import { cancelAgentRun, createAgentRun, getAgentRun, retryAgentRun } from '@/features/agent-run/api';
import { queryKeys } from '@/api/queryKeys';
import { listKnowledgeBases } from '@/features/knowledge-base/api';
import { ChatWorkspace } from '@/layouts/ChatWorkspace';
import { createConversation, listMessages, updateConversation } from '../api';
import { streamAgentEvents } from '../events';
import { ChatComposer } from '../components/ChatComposer';
import { MessageList } from '../components/MessageList';
import type { AgentRunViewState } from '@/features/agent-run/types';
import type { Citation, Message } from '../types';
import { v4 as uuidv4 } from 'uuid';

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

/**
 * Merge consecutive assistant messages into a single message with version history.
 * When multiple AI replies appear back-to-back (no user message between them),
 * they are automatically grouped into one message with version tabs.
 */
function mergeConsecutiveAssistantMessages(messages: Message[]): Message[] {
  if (messages.length <= 1) return messages;

  const result: Message[] = [];
  let i = 0;

  while (i < messages.length) {
    const current = messages[i];

    if (current.role === 'assistant' && i + 1 < messages.length && messages[i + 1].role === 'assistant') {
      // Found consecutive assistant messages — merge all of them into one
      const consecutive: Message[] = [current];
      let j = i + 1;
      while (j < messages.length && messages[j].role === 'assistant') {
        consecutive.push(messages[j]);
        j++;
      }

      // The last one in the group is the primary (latest); all earlier ones become versions
      const primary = consecutive[consecutive.length - 1];
      const earlierVersions = consecutive.slice(0, -1).map((m) => ({
        content: m.content,
        agent_run_id: m.agent_run_id!,
        status: m.status,
        created_at: m.created_at,
      }));

      result.push({
        ...primary,
        versions: [...(primary.versions || []), ...earlierVersions],
        current_version_index: primary.current_version_index ?? -1,
      });

      i = j;
    } else {
      result.push(current);
      i++;
    }
  }

  return result;
}

function ChatPageContent({ kbId, conversationId }: { kbId: string; conversationId?: string }) {
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
  const activeConversationIdRef = useRef<string | undefined>(conversationId);
  activeConversationIdRef.current = activeConversationId;
  const conversationRunIdsRef = useRef<Record<string, string>>({});
  const pendingConversationNavigationRef = useRef<string | null>(null);
  const [activeRunId, setActiveRunId] = useState<string | null>(null);
  const [retryingMessageId, setRetryingMessageId] = useState<string | null>(null);
  const [resumingRun, setResumingRun] = useState(false);
  const [historicalRunStates, setHistoricalRunStates] = useState<Record<string, AgentRunViewState>>({});
  const historicalHydratedRef = useRef<Set<string>>(new Set());

  /**
   * Wrapper around setMessages that auto-applies merge logic.
   */
  const updateMessages = (updater: Message[] | ((prev: Message[]) => Message[])) => {
    setMessages((prev) => {
      const next = typeof updater === 'function' ? updater(prev) : updater;
      return mergeConsecutiveAssistantMessages(next);
    });
  };

  useEffect(() => {
    if (conversationId && pendingConversationNavigationRef.current === conversationId) {
      pendingConversationNavigationRef.current = null;
      return;
    }
    // 只断开当前页面的事件订阅，不取消服务端仍在执行的 Agent。
    abortRef.current?.abort();
    abortRef.current = null;
    replayAbortRef.current?.abort();
    replayAbortRef.current = null;
    setActiveConversationId(conversationId);
    updateMessages([]);
    currentRunId.current = null;
    setActiveRunId(null);
    setRetryingMessageId(null);
    setResumingRun(false);
    setSubmitting(false);
    hydratedRunIdRef.current = null;
    dispatchRun({ type: 'RESET_AGENT_RUN_STATE' });
    setHistoricalRunStates({});
    historicalHydratedRef.current = new Set();
  }, [kbId, conversationId]);

  useEffect(() => () => {
    abortRef.current?.abort();
    replayAbortRef.current?.abort();
  }, []);

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

  // Load messages from API — applying auto-merge on consecutive assistant messages
  useEffect(() => {
    if (!messagesQuery.data) return;
    const sortedMessages = [...messagesQuery.data.items].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    updateMessages(sortedMessages);
  }, [messagesQuery.data]);

  // Replay / resume agent run state when loading messages from API
  useEffect(() => {
    if (!messagesQuery.data) return;

    const sortedMessages = [...messagesQuery.data.items].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    const latestRunId = (activeConversationId ? conversationRunIdsRef.current[activeConversationId] : undefined)
      || [...sortedMessages].reverse().find((message) => message.agent_run_id)?.agent_run_id;
    if (!latestRunId || hydratedRunIdRef.current === latestRunId) return;

    const replayConversationId = activeConversationId;
    replayAbortRef.current?.abort();
    const controller = new AbortController();
    replayAbortRef.current = controller;
    hydratedRunIdRef.current = latestRunId;
    currentRunId.current = latestRunId;
    setActiveRunId(latestRunId);
    setResumingRun(true);
    dispatchRun({ type: 'RESET_AGENT_RUN_STATE' });

    void (async () => {
      try {
        const run = await getAgentRun(latestRunId);
        if (controller.signal.aborted) return;
        dispatchRun({ type: 'HYDRATE_AGENT_RUN_STATE', run });
        const terminal = run.status === 'completed' || run.status === 'failed' || run.status === 'cancelled';
        if (!terminal && activeConversationIdRef.current === replayConversationId) setSubmitting(true);
        await streamAgentEvents(`${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${latestRunId}/events`, {
          signal: controller.signal,
          afterSequence: 0,
          onEvent: (event) => {
            if (activeConversationIdRef.current === replayConversationId) dispatchRun(event);
          },
          timeout: terminal ? 5000 : undefined,
        });

        // If the run was NOT terminal when we started replaying,
        // it may have just become terminal during streaming —
        // add the assistant message if it hasn't been added yet
        if (!terminal && !controller.signal.aborted && activeConversationIdRef.current === replayConversationId) {
          const completedRun = await getAgentRun(latestRunId);
          const answer = completedRun.final_result ?? runStateRef.current.answer ?? '';
          if (answer !== undefined && answer !== null && activeConversationIdRef.current === replayConversationId) {
            updateMessages((current) => {
              if (current.some((m) => m.agent_run_id === latestRunId && m.role === 'assistant')) return current;
              return [...current, {
                id: uuidv4(),
                role: 'assistant',
                content: answer,
                agent_run_id: latestRunId,
                status: completedRun.status || 'completed',
                citations: normalizeCitations(runStateRef.current.citations),
                created_at: new Date().toISOString(),
              }];
            });
          }
        }
      } catch (error) {
        if (!controller.signal.aborted) setErrorMessage(error instanceof Error ? error.message : 'Agent 运行轨迹恢复失败');
      } finally {
        if (replayAbortRef.current === controller) replayAbortRef.current = null;
        setResumingRun(false);
        if (activeConversationIdRef.current === replayConversationId && currentRunId.current === latestRunId) {
          setSubmitting(false);
        }
      }
    })();

    return () => controller.abort();
  }, [messagesQuery.data, activeConversationId]);

  // Replay ALL historical (non-latest) agent runs to populate historical run states
  useEffect(() => {
    if (!messagesQuery.data || !activeConversationId) return;

    const sortedMessages = [...messagesQuery.data.items].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    const allRunIds = [...new Set(sortedMessages
      .filter((m) => m.role === 'assistant' && m.agent_run_id)
      .map((m) => m.agent_run_id))] as string[];

    const latestRunId = allRunIds[allRunIds.length - 1];
    const historicalRunIds = allRunIds.filter((id) => id !== latestRunId);

    const hydrateConversationId = activeConversationId;
    const controller = new AbortController();

    void (async () => {
      for (const runId of historicalRunIds) {
        if (controller.signal.aborted) break;
        if (historicalHydratedRef.current.has(runId)) continue;
        historicalHydratedRef.current.add(runId);

        let state = initialAgentRunState;
        try {
          await streamAgentEvents(`${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${runId}/events`, {
            signal: controller.signal,
            afterSequence: 0,
            onEvent: (event) => {
              if (activeConversationIdRef.current === hydrateConversationId) {
                state = reduceAgentEvent(state, event);
              }
            },
            timeout: 10000,
          });
        } catch {
          // Stream ended or timeout — use whatever state was accumulated
        }
        if (activeConversationIdRef.current === hydrateConversationId) {
          setHistoricalRunStates((prev) => {
            if (prev[runId] === state) return prev;
            return { ...prev, [runId]: state };
          });
        }
      }
    })();

    return () => controller.abort();
  }, [messagesQuery.data, activeConversationId]);

  // Sync the live agent run state into historical run states for display
  useEffect(() => {
    if (activeRunId && runState.status !== 'idle') {
      setHistoricalRunStates((prev) => {
        if (prev[activeRunId] === runState) return prev;
        return { ...prev, [activeRunId]: runState };
      });
    }
  }, [runState, activeRunId]);

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

  const updateConversationTitle = (id: string, title: string) => {
    void updateConversation(id, title);
    void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
  };

  const runAgentStream = async (runId: string, opts?: { replaceMessageId?: string }) => {
    replayAbortRef.current?.abort();
    const streamConversationId = activeConversationIdRef.current;
    currentRunId.current = runId;
    setActiveRunId(runId);
    hydratedRunIdRef.current = runId;
    setResumingRun(false);
    dispatchRun({ type: 'RESET_AGENT_RUN_STATE' });
    dispatchRun({ type: 'SET_AGENT_RUN_QUEUED' });
    const controller = new AbortController();
    abortRef.current = controller;
    await streamAgentEvents(`${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${runId}/events`, {
      signal: controller.signal,
      onEvent: (event) => {
        if (activeConversationIdRef.current === streamConversationId) dispatchRun(event);
      },
    });
    const completedRun = await getAgentRun(runId);
    const answer = completedRun.final_result ?? runStateRef.current.answer ?? '';
    if (answer !== undefined && answer !== null && activeConversationIdRef.current === streamConversationId) {
      updateMessages((current) => {
        const newMessage: Message = {
          id: uuidv4(),
          role: 'assistant',
          content: answer,
          agent_run_id: runId,
          status: completedRun.status || 'completed',
          citations: normalizeCitations(runStateRef.current.citations),
          created_at: new Date().toISOString(),
        };

        if (opts?.replaceMessageId) {
          // Replace old message — save old content as a version first
          const idx = current.findIndex((m) => m.id === opts.replaceMessageId);
          if (idx !== -1) {
            const old = current[idx];
            const oldVersion = {
              content: old.content,
              agent_run_id: old.agent_run_id || '',
              status: old.status,
              created_at: old.created_at,
            };
            newMessage.versions = [...(old.versions || []), oldVersion];
            newMessage.current_version_index = -1; // -1 means showing latest
            const updated = [...current];
            updated[idx] = newMessage;
            return updated;
          }
        }

        // Guard: avoid appending a duplicate assistant message for the same run
        if (current.some((m) => m.agent_run_id === runId && m.role === 'assistant')) return current;
        return [...current, newMessage];
      });
    }
  };

  const send = async () => {
    if (!enabled || !draft.trim() || submitting) return;
    const query = draft.trim();
    setSubmitting(true);

    try {
      setErrorMessage(null);
      const id = await getConversationId();
      setDraft('');

      const userMessageId = uuidv4();
      updateMessages((current) => [...current, {
        id: userMessageId, role: 'user', content: query, agent_run_id: null, created_at: new Date().toISOString(),
      }]);

      if (!conversationId) {
        updateConversationTitle(id, query);
      }

      const response = await createAgentRun({ knowledge_base_id: kbId, conversation_id: id, query });
      replayAbortRef.current?.abort();
      // 记录 run_id，切换会话返回时 replay 逻辑才能恢复运行中的状态
      conversationRunIdsRef.current[id] = response.run_id;
      const streamConversationId = id;
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
        onEvent: (event) => {
          if (activeConversationIdRef.current === streamConversationId) dispatchRun(event);
        },
      });
        const completedRun = await getAgentRun(response.run_id);
      // Plan-Execute: SSE 完成事件可能先于 DB 写入，短暂延时后重试
      let answer = completedRun.final_result || runStateRef.current.answer;
      if (!answer) {
        await new Promise(r => setTimeout(r, 500));
        const retried = await getAgentRun(response.run_id);
        answer = retried.final_result || runStateRef.current.answer;
      }
      if (answer && activeConversationIdRef.current === streamConversationId) {
        updateMessages((current) => {
          // Guard: 避免同一 run 重复追加助手消息（切换会话后可能被 replay 逻辑重复写入）
          if (current.some((m) => m.agent_run_id === response.run_id && m.role === 'assistant')) return current;
          return [...current, {
            id: uuidv4(), role: 'assistant', content: answer, agent_run_id: response.run_id,
            status: completedRun.status || 'completed', citations: normalizeCitations(runStateRef.current.citations), created_at: new Date().toISOString(),
          }];
        });
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

  const retry = async (agentRunId: string) => {
    if (!enabled || submitting) return;
    setSubmitting(true);

    try {
      setErrorMessage(null);
      const response = await retryAgentRun(agentRunId);
      const id = activeConversationId || await getConversationId();
      conversationRunIdsRef.current[id] = response.new_run_id;

      // Find the assistant message being retried and remember its ID for replacement
      const msgs = messages;
      const lastAssistantIdx = [...msgs].reverse().findIndex((m) => m.role === 'assistant' && m.agent_run_id === agentRunId);
      const replaceMsgId = lastAssistantIdx !== -1 ? msgs[msgs.length - 1 - lastAssistantIdx].id : undefined;
      setRetryingMessageId(replaceMsgId ?? null);

      await runAgentStream(response.new_run_id, { replaceMessageId: replaceMsgId });
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '重试请求失败');
    } finally {
      void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
      if (activeConversationId) void queryClient.invalidateQueries({ queryKey: ['conversations', activeConversationId, 'messages'] });
      setSubmitting(false);
      setRetryingMessageId(null);
      abortRef.current = null;
    }
  };

  const switchVersion = (messageId: string, versionIdx: number) => {
    updateMessages((current) => current.map((m) =>
      m.id === messageId ? { ...m, current_version_index: versionIdx } : m,
    ));
  };

  const stop = () => {
    abortRef.current?.abort();
    if (currentRunId.current) {
      void cancelAgentRun(currentRunId.current).then(() => {
        dispatchRun({ type: 'SET_AGENT_RUN_CANCELLED' });
      });
    }
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
  // 合并活跃运行状态和历史运行状态，确保切换版本时能找到对应 agent_run_id 的运行记录。
  const allRunStates = {
    ...(activeRunId && runState.status !== 'idle' ? { [activeRunId]: runState } : {}),
    ...historicalRunStates,
  };

  const empty = messages.length === 0 && !submitting;
  const messageArea = (
    <Stack sx={{ flex: 1, minHeight: 0 }}>
      {!enabled && <Alert severity="info" sx={{ m: 2, mb: 0 }}>智能问答后端未启用，请检查服务配置。</Alert>}
      {errorMessage && <Alert severity="error" sx={{ m: 2, mb: 0 }}>{errorMessage}</Alert>}
      {messagesQuery.error && <Alert severity="warning" sx={{ m: 2, mb: 0 }}>历史消息加载失败，请稍后重试。</Alert>}
      <MessageList messages={messages} streamingAnswer={submitting && !resumingRun ? runState.answer : ''} agentRunState={runState} agentRunId={activeRunId} agentRunStates={allRunStates} retryingMessageId={retryingMessageId} resumingRun={resumingRun} emptyComposer={empty ? composer : undefined} onSuggestion={setDraft} onRetry={retry} onSwitchVersion={switchVersion} />
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
