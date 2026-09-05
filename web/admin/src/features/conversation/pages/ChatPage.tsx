import { Alert, Button, Stack } from '@mui/material';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { capabilities } from '@/app/capabilities';
import { initialAgentRunState, reduceAgentEvent } from '@/features/agent-run/eventReducer';
import { cancelAgentRun, createAgentRun, getAgentRun, retryAgentRun } from '@/features/agent-run/api';
import { queryKeys } from '@/api/queryKeys';
import { listKnowledgeBases } from '@/features/knowledge-base/api';
import { listModelConfigs } from '@/features/model/api';
import { ChatWorkspace } from '@/layouts/ChatWorkspace';
import { ActionNotice } from '@/components/shared/ActionNotice';
import { createConversation, getConversation, listMessages, updateConversation, updateConversationChatModel } from '../api';
import { streamAgentEvents } from '../events';
import { ChatComposer } from '../components/ChatComposer';
import { MessageList } from '../components/MessageList';
import type { Citation } from '../types';
import { v4 as uuidv4 } from 'uuid';
import { useAppDispatch, useAppSelector } from '@/store';
import {
  applyRunAction,
  appendAssistantMessage,
  appendUserMessage,
  markActiveRunHydrated,
  markHistoricalHydrated,
  setConversationRunId,
  setHistoricalRunState,
  setMessages,
  switchMessageVersion,
} from '@/store/agentRunSlice';

const conversationStorageKey = (kbId: string) => `memora:conversation:${kbId}`;

function normalizeCitations(values: Array<Record<string, unknown>>): Citation[] {
  return values.map((value) => ({
    source_type: value.source_type === 'network' ? 'network' : 'knowledge_base',
    knowledge_base_id: typeof value.knowledge_base_id === 'string' ? value.knowledge_base_id : undefined,
    document_id: typeof value.document_id === 'string' ? value.document_id : undefined,
    document_title: typeof value.document_title === 'string' ? value.document_title : undefined,
    quoted_text: typeof value.quoted_text === 'string' ? value.quoted_text : undefined,
    title: typeof value.title === 'string' ? value.title : undefined,
    url: typeof value.url === 'string' ? value.url : undefined,
    site_name: typeof value.site_name === 'string' ? value.site_name : undefined,
  }));
}

const runEventsUrl = (runId: string) =>
  `${import.meta.env.VITE_API_BASE_URL || '/api/v1'}/agent/runs/${runId}/events`;

const isTerminalStatus = (status: string | undefined) =>
  status === 'completed' || status === 'failed' || status === 'cancelled';

function ChatPageContent({ kbId, conversationId }: { kbId: string; conversationId?: string }) {
  const enabled = capabilities.conversation === 'available';
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const dispatch = useAppDispatch();

  // 全局 store 中按会话键控的 Agent 运行状态与消息（跨路由存活）
  const agentRunStatesStore = useAppSelector((state) => state.agentRun.runStates);
  const storeMessages = useAppSelector((state) => state.agentRun.messages);
  const conversationRunIds = useAppSelector((state) => state.agentRun.conversationRunIds);
  const storeHistoricalRunStates = useAppSelector((state) => state.agentRun.historicalRunStates);
  const storeHistoricalHydrated = useAppSelector((state) => state.agentRun.historicalHydratedIds);
  const activeHydratedRunIds = useAppSelector((state) => state.agentRun.activeHydratedRunIds);

  const [draft, setDraft] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [activeConversationId, setActiveConversationId] = useState(conversationId);
  const [retryingMessageId, setRetryingMessageId] = useState<string | null>(null);
  const [resumingRun, setResumingRun] = useState(false);
  const [selectedChatModelId, setSelectedChatModelId] = useState('');
  const [modelNotice, setModelNotice] = useState('');

  const abortRef = useRef<AbortController | null>(null);
  const replayAbortRef = useRef<AbortController | null>(null);
  const currentRunId = useRef<string | null>(null);
  const activeConversationIdRef = useRef<string | undefined>(conversationId);
  activeConversationIdRef.current = activeConversationId;
  const pendingConversationNavigationRef = useRef<string | null>(null);

  // 供异步回调读取最新 store 状态的镜像 ref（避免闭包过期）
  const agentRunStatesRef = useRef(agentRunStatesStore);
  agentRunStatesRef.current = agentRunStatesStore;
  const conversationRunIdsRef = useRef(conversationRunIds);
  conversationRunIdsRef.current = conversationRunIds;
  const historicalHydratedRef = useRef(storeHistoricalHydrated);
  historicalHydratedRef.current = storeHistoricalHydrated;
  const activeHydratedRunIdsRef = useRef(activeHydratedRunIds);
  activeHydratedRunIdsRef.current = activeHydratedRunIds;

  // 当前会话的派生状态
  const runState = activeConversationId ? (agentRunStatesStore[activeConversationId] ?? initialAgentRunState) : initialAgentRunState;
  const messages = activeConversationId ? (storeMessages[activeConversationId] ?? []) : [];
  const historical = activeConversationId ? (storeHistoricalRunStates[activeConversationId] ?? {}) : {};
  const activeRunId = activeConversationId
    ? (conversationRunIds[activeConversationId] ?? [...messages].reverse().find((m) => m.agent_run_id)?.agent_run_id ?? null)
    : null;

  // 是否正在运行的派生状态：直接复用 store 中按会话键控的 runState.status，
  // 避免依赖易被切换逻辑重置的局部 submitting，保证暂停按钮状态不丢失且各会话互不干扰。
  const isAgentLive = runState.status === 'queued' || runState.status === 'running';

  /**
   * 运行结束后收尾：若缺少助手消息则补上（按 run_id 去重）、标记该 run 已完整回放、触发消息刷新。
   */
  const finalizeRun = async (targetConversationId: string, runId: string) => {
    const completedRun = await getAgentRun(runId);
    const state = agentRunStatesRef.current[targetConversationId];
    const answer = completedRun.final_result ?? state?.answer ?? '';
    if (answer && activeConversationIdRef.current === targetConversationId) {
      dispatch(appendAssistantMessage({
        conversationId: targetConversationId,
        message: {
          id: uuidv4(),
          role: 'assistant',
          content: answer,
          agent_run_id: runId,
          status: completedRun.status || 'completed',
          citations: normalizeCitations(state?.citations ?? []),
          created_at: new Date().toISOString(),
        },
      }));
    }
    dispatch(markActiveRunHydrated({ conversationId: targetConversationId, runId }));
    void queryClient.invalidateQueries({ queryKey: ['conversations', targetConversationId, 'messages'] });
  };

  useEffect(() => {
    if (conversationId && pendingConversationNavigationRef.current === conversationId) {
      pendingConversationNavigationRef.current = null;
      return;
    }
    // 只断开当前页面的事件订阅，不取消服务端仍在执行的 Agent，也不清空全局运行状态。
    abortRef.current?.abort();
    abortRef.current = null;
    replayAbortRef.current?.abort();
    replayAbortRef.current = null;
    setActiveConversationId(conversationId);
    setRetryingMessageId(null);
    setResumingRun(false);
    setSubmitting(false);
    setErrorMessage(null);
    setSelectedChatModelId('');
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

  const modelsQuery = useQuery({
    queryKey: [...queryKeys.models, 'chat'],
    queryFn: () => listModelConfigs({ model_type: 'chat' }),
    enabled,
    staleTime: 30_000,
  });
  const chatModels = useMemo(() => (modelsQuery.data?.items ?? []).filter((model) => model.enabled), [modelsQuery.data?.items]);

  const conversationQuery = useQuery({
    queryKey: ['conversations', activeConversationId],
    queryFn: () => getConversation(activeConversationId as string),
    enabled: enabled && Boolean(activeConversationId),
  });

  useEffect(() => {
    if (conversationQuery.data) {
      if (!modelsQuery.data) return;
      const currentModelAvailable = chatModels.some((model) => model.id === conversationQuery.data.chat_model_id);
      setSelectedChatModelId(currentModelAvailable ? conversationQuery.data.chat_model_id : '');
      return;
    }
    if (!activeConversationId && !selectedChatModelId && chatModels.length > 0) {
      const preferred = chatModels.find((model) => model.is_default) ?? chatModels[0];
      setSelectedChatModelId(preferred.id);
    }
  }, [conversationQuery.data, modelsQuery.data, activeConversationId, selectedChatModelId, chatModels]);

  const messagesQuery = useQuery({
    queryKey: ['conversations', activeConversationId, 'messages'],
    queryFn: () => listMessages(activeConversationId as string, { page: 1, page_size: 100 }),
    enabled: enabled && Boolean(activeConversationId) && !submitting,
  });

  // Load messages from API — only when store 中尚无该会话消息，避免乐观写入被缓存覆盖
  useEffect(() => {
    if (!messagesQuery.data || !activeConversationId) return;
    if (storeMessages[activeConversationId] && storeMessages[activeConversationId].length > 0) return;
    const sortedMessages = [...messagesQuery.data.items].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    dispatch(setMessages({ conversationId: activeConversationId, messages: sortedMessages }));
  }, [messagesQuery.data, activeConversationId, storeMessages, dispatch]);

  // 续传：进入一个已在运行中的会话时，从断点继续订阅，不清空、不重放、不显示加载态。
  useEffect(() => {
    if (!activeConversationId) return;
    const storedRunId = conversationRunIdsRef.current[activeConversationId];
    const existing = agentRunStatesRef.current[activeConversationId];
    if (!storedRunId) return;
    if (!existing || (existing.status !== 'queued' && existing.status !== 'running')) return;

    const replayConversationId = activeConversationId;
    replayAbortRef.current?.abort();
    const controller = new AbortController();
    replayAbortRef.current = controller;
    setSubmitting(true);
    setResumingRun(false);

    void (async () => {
      try {
        await streamAgentEvents(runEventsUrl(storedRunId), {
          signal: controller.signal,
          afterSequence: existing.highest_sequence,
          onEvent: (event) => {
            if (activeConversationIdRef.current === replayConversationId) {
              dispatch(applyRunAction({ conversationId: replayConversationId, runAction: event }));
            }
          },
        });
        const finalStatus = agentRunStatesRef.current[replayConversationId]?.status;
        if (isTerminalStatus(finalStatus)) {
          await finalizeRun(replayConversationId, storedRunId);
        }
      } catch (error) {
        if (!controller.signal.aborted) setErrorMessage(error instanceof Error ? error.message : 'Agent 运行轨迹恢复失败');
      } finally {
        if (replayAbortRef.current === controller) replayAbortRef.current = null;
        if (activeConversationIdRef.current === replayConversationId) setSubmitting(false);
      }
    })();

    return () => controller.abort();
  }, [activeConversationId, dispatch]);

  // 完整回放：首次进入（store 无该 run 状态）时从 0 拉取最新 run 轨迹。
  useEffect(() => {
    if (!messagesQuery.data || !activeConversationId) return;

    const sortedMessages = [...messagesQuery.data.items].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    const latestRunId = conversationRunIdsRef.current[activeConversationId]
      || [...sortedMessages].reverse().find((message) => message.agent_run_id)?.agent_run_id;
    if (!latestRunId) return;

    const existing = agentRunStatesRef.current[activeConversationId];
    const existingIsLive = existing && (existing.status === 'queued' || existing.status === 'running');
    if (existingIsLive) return; // 已由续传 effect 处理
    if (activeHydratedRunIdsRef.current[activeConversationId] === latestRunId) return;

    const replayConversationId = activeConversationId;
    replayAbortRef.current?.abort();
    const controller = new AbortController();
    replayAbortRef.current = controller;
    dispatch(setConversationRunId({ conversationId: replayConversationId, runId: latestRunId }));
    setResumingRun(true);

    void (async () => {
      try {
        const run = await getAgentRun(latestRunId);
        if (controller.signal.aborted) return;
        dispatch(applyRunAction({ conversationId: replayConversationId, runAction: { type: 'HYDRATE_AGENT_RUN_STATE', run } }));
        const terminal = isTerminalStatus(run.status);
        if (!terminal) setSubmitting(true);
        await streamAgentEvents(runEventsUrl(latestRunId), {
          signal: controller.signal,
          afterSequence: 0,
          onEvent: (event) => {
            if (activeConversationIdRef.current === replayConversationId) {
              dispatch(applyRunAction({ conversationId: replayConversationId, runAction: event }));
            }
          },
          timeout: terminal ? 5000 : undefined,
        });

        if (!terminal && !controller.signal.aborted && activeConversationIdRef.current === replayConversationId) {
          await finalizeRun(replayConversationId, latestRunId);
        } else if (activeConversationIdRef.current === replayConversationId) {
          dispatch(markActiveRunHydrated({ conversationId: replayConversationId, runId: latestRunId }));
        }
      } catch (error) {
        if (!controller.signal.aborted) setErrorMessage(error instanceof Error ? error.message : 'Agent 运行轨迹恢复失败');
      } finally {
        if (replayAbortRef.current === controller) replayAbortRef.current = null;
        setResumingRun(false);
        if (activeConversationIdRef.current === replayConversationId) setSubmitting(false);
      }
    })();

    return () => controller.abort();
  }, [messagesQuery.data, activeConversationId, dispatch]);

  // 回放所有历史（非最新）agent run，用于版本切换时展示对应运行轨迹。
  useEffect(() => {
    if (!messagesQuery.data || !activeConversationId) return;
    if (historicalHydratedRef.current[activeConversationId]) return;

    const sortedMessages = [...messagesQuery.data.items].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
    const allRunIds = [...new Set(sortedMessages
      .filter((m) => m.role === 'assistant' && m.agent_run_id)
      .map((m) => m.agent_run_id))] as string[];

    const latestRunId = allRunIds[allRunIds.length - 1];
    const historicalRunIds = allRunIds.filter((id) => id !== latestRunId);

    if (historicalRunIds.length === 0) {
      dispatch(markHistoricalHydrated(activeConversationId));
      return;
    }

    const hydrateConversationId = activeConversationId;
    const controller = new AbortController();

    void (async () => {
      for (const runId of historicalRunIds) {
        if (controller.signal.aborted) break;
        let state = initialAgentRunState;
        try {
          await streamAgentEvents(runEventsUrl(runId), {
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
          dispatch(setHistoricalRunState({ conversationId: hydrateConversationId, runId, runState: state }));
        }
      }
      if (activeConversationIdRef.current === hydrateConversationId) {
        dispatch(markHistoricalHydrated(hydrateConversationId));
      }
    })();

    return () => controller.abort();
  }, [messagesQuery.data, activeConversationId, dispatch]);

  const getConversationId = async () => {
    if (activeConversationId) return activeConversationId;
    if (!selectedChatModelId) throw new Error('请先选择可用的 Chat 模型');
    const conversation = await createConversation(kbId, '新会话', selectedChatModelId);
    sessionStorage.setItem(conversationStorageKey(kbId), conversation.id);
    pendingConversationNavigationRef.current = conversation.id;
    setActiveConversationId(conversation.id);
    navigate(`/chat/${kbId}/${conversation.id}`, { replace: true });
    void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
    return conversation.id;
  };

  const changeChatModel = async (chatModelId: string) => {
    if (!chatModelId || chatModelId === selectedChatModelId) return;
    try {
      setErrorMessage(null);
      if (activeConversationId) {
        await updateConversationChatModel(activeConversationId, chatModelId);
        await queryClient.invalidateQueries({ queryKey: ['conversations', activeConversationId] });
      }
      setSelectedChatModelId(chatModelId);
      setModelNotice('Chat 模型已切换，后续请求将使用该模型');
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '切换 Chat 模型失败');
    }
  };

  const updateConversationTitle = (id: string, title: string) => {
    void updateConversation(id, title);
    void queryClient.invalidateQueries({ queryKey: queryKeys.conversations(kbId) });
  };

  const runAgentStream = async (runId: string, opts?: { replaceMessageId?: string }) => {
    replayAbortRef.current?.abort();
    const streamConversationId = activeConversationIdRef.current;
    if (!streamConversationId) return;
    currentRunId.current = runId;
    setResumingRun(false);
    dispatch(setConversationRunId({ conversationId: streamConversationId, runId }));
    dispatch(applyRunAction({ conversationId: streamConversationId, runAction: { type: 'RESET_AGENT_RUN_STATE' } }));
    dispatch(applyRunAction({ conversationId: streamConversationId, runAction: { type: 'SET_AGENT_RUN_QUEUED' } }));
    const controller = new AbortController();
    abortRef.current = controller;
    await streamAgentEvents(runEventsUrl(runId), {
      signal: controller.signal,
      onEvent: (event) => {
        if (activeConversationIdRef.current === streamConversationId) {
          dispatch(applyRunAction({ conversationId: streamConversationId, runAction: event }));
        }
      },
    });
    const completedRun = await getAgentRun(runId);
    const answer = completedRun.final_result ?? agentRunStatesRef.current[streamConversationId]?.answer ?? '';
    if (answer !== undefined && answer !== null && activeConversationIdRef.current === streamConversationId) {
      dispatch(appendAssistantMessage({
        conversationId: streamConversationId,
        message: {
          id: uuidv4(),
          role: 'assistant',
          content: answer,
          agent_run_id: runId,
          status: completedRun.status || 'completed',
          citations: normalizeCitations(agentRunStatesRef.current[streamConversationId]?.citations ?? []),
          created_at: new Date().toISOString(),
        },
        replaceMessageId: opts?.replaceMessageId,
      }));
    }
    // 正常完整跑完（未被中断）时，标记该 run 已完成回放，再次进入会话直接复用 store 状态，避免中间加载态。
    if (!controller.signal.aborted) {
      dispatch(markActiveRunHydrated({ conversationId: streamConversationId, runId }));
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
      dispatch(appendUserMessage({ conversationId: id, message: { id: userMessageId, role: 'user', content: query, agent_run_id: null, created_at: new Date().toISOString() } }));

      if (!conversationId) {
        updateConversationTitle(id, query);
      }

      const response = await createAgentRun({ knowledge_base_id: kbId, conversation_id: id, query });
      replayAbortRef.current?.abort();
      // 记录 run_id，切换会话返回时续传逻辑才能恢复运行中的状态
      dispatch(setConversationRunId({ conversationId: id, runId: response.run_id }));
      const streamConversationId = id;
      currentRunId.current = response.run_id;
      // 重置该会话的 reducer 状态，清除上一轮运行的数据（highest_sequence、answer 等）
      dispatch(applyRunAction({ conversationId: id, runAction: { type: 'RESET_AGENT_RUN_STATE' } }));
      dispatch(applyRunAction({ conversationId: id, runAction: { type: 'SET_AGENT_RUN_QUEUED' } }));
      const controller = new AbortController();
      abortRef.current = controller;
      await streamAgentEvents(runEventsUrl(response.run_id), {
        signal: controller.signal,
        onEvent: (event) => {
          if (activeConversationIdRef.current === streamConversationId) {
            dispatch(applyRunAction({ conversationId: streamConversationId, runAction: event }));
          }
        },
      });
      const completedRun = await getAgentRun(response.run_id);
      // Plan-Execute: SSE 完成事件可能先于 DB 写入，短暂延时后重试
      let answer = completedRun.final_result || agentRunStatesRef.current[streamConversationId]?.answer;
      if (!answer) {
        await new Promise((r) => setTimeout(r, 500));
        const retried = await getAgentRun(response.run_id);
        answer = retried.final_result || agentRunStatesRef.current[streamConversationId]?.answer;
      }
      if (answer && activeConversationIdRef.current === streamConversationId) {
        dispatch(appendAssistantMessage({
          conversationId: streamConversationId,
          message: {
            id: uuidv4(), role: 'assistant', content: answer, agent_run_id: response.run_id,
            status: completedRun.status || 'completed', citations: normalizeCitations(agentRunStatesRef.current[streamConversationId]?.citations ?? []), created_at: new Date().toISOString(),
          },
        }));
      }
      // 正常完整跑完（未被中断）时，标记该 run 已完成回放，再次进入会话直接复用 store 状态，避免中间加载态。
      if (!controller.signal.aborted) {
        dispatch(markActiveRunHydrated({ conversationId: streamConversationId, runId: response.run_id }));
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
      dispatch(setConversationRunId({ conversationId: id, runId: response.new_run_id }));

      // Find the assistant message being retried and remember its ID for replacement
      const msgs = storeMessages[id] ?? [];
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
    if (activeConversationId) {
      dispatch(switchMessageVersion({ conversationId: activeConversationId, messageId, versionIdx }));
    }
  };

  const stop = () => {
    abortRef.current?.abort();
    if (currentRunId.current) {
      void cancelAgentRun(currentRunId.current).then(() => {
        if (activeConversationIdRef.current) {
          dispatch(applyRunAction({ conversationId: activeConversationIdRef.current, runAction: { type: 'SET_AGENT_RUN_CANCELLED' } }));
        }
      });
    }
  };

  const composer = (
    <ChatComposer
      draft={draft}
      disabled={!enabled || !selectedChatModelId}
      streaming={submitting || isAgentLive}
      knowledgeBases={knowledgeBasesQuery.data?.items ?? []}
      selectedKnowledgeBaseId={kbId}
      chatModels={chatModels}
      selectedChatModelId={selectedChatModelId}
      onDraftChange={setDraft}
      onKnowledgeBaseChange={(knowledgeBaseId) => navigate(`/chat/${knowledgeBaseId}`)}
      onChatModelChange={(chatModelId) => void changeChatModel(chatModelId)}
      onSend={() => void send()}
      onStop={stop}
    />
  );
  // 合并活跃运行状态和历史运行状态，确保切换版本时能找到对应 agent_run_id 的运行记录。
  const allRunStates = {
    ...(activeRunId && runState.status !== 'idle' ? { [activeRunId]: runState } : {}),
    ...historical,
  };

  const empty = messages.length === 0 && !submitting;
  const messageArea = (
    <Stack sx={{ flex: 1, minHeight: 0 }}>
      {!enabled && <Alert severity="info" sx={{ m: 2, mb: 0 }}>智能问答后端未启用，请检查服务配置。</Alert>}
      {!modelsQuery.isPending && chatModels.length === 0 && (
        <Alert severity="warning" sx={{ m: 2, mb: 0 }} action={<Button component={Link} to="/settings/models" size="small">前往配置</Button>}>
          暂无可用的 Chat 模型，请先完成模型配置。
        </Alert>
      )}
      {errorMessage && <Alert severity="error" sx={{ m: 2, mb: 0 }}>{errorMessage}</Alert>}
      {messagesQuery.error && <Alert severity="warning" sx={{ m: 2, mb: 0 }}>历史消息加载失败，请稍后重试。</Alert>}
      <MessageList messages={messages} knowledgeBaseId={kbId} streamingAnswer={submitting && !resumingRun ? runState.answer : ''} agentRunState={runState} agentRunId={activeRunId} agentRunStates={allRunStates} retryingMessageId={retryingMessageId} resumingRun={resumingRun} emptyComposer={empty ? composer : undefined} onSuggestion={setDraft} onRetry={retry} onSwitchVersion={switchVersion} />
    </Stack>
  );

  return (
    <>
      <ChatWorkspace messages={messageArea} composer={empty ? null : composer} />
      <ActionNotice message={modelNotice} onClose={() => setModelNotice('')} />
    </>
  );
}

export function ChatPage() {
  const { kbId = '', conversationId } = useParams();
  return <ChatPageContent kbId={kbId} conversationId={conversationId} />;
}
