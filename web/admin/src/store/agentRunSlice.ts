import { createSlice, type PayloadAction } from '@reduxjs/toolkit';
import { initialAgentRunState, reduceAgentEvent, type AgentRunAction } from '@/features/agent-run/eventReducer';
import type { AgentRunViewState } from '@/features/agent-run/types';
import type { Message, MessageVersion } from '@/features/conversation/types';

/**
 * 合并连续的助手消息为单条消息 + 版本历史（重试产生的多个 AI 回复）。
 * 与 ChatPage 原有 mergeConsecutiveAssistantMessages 逻辑一致，迁入 store 以便 reducer 内完成合并。
 */
function mergeConsecutiveAssistantMessages(messages: Message[]): Message[] {
  if (messages.length <= 1) return messages;

  const result: Message[] = [];
  let i = 0;

  while (i < messages.length) {
    const current = messages[i];

    if (current.role === 'assistant' && i + 1 < messages.length && messages[i + 1].role === 'assistant') {
      const consecutive: Message[] = [current];
      let j = i + 1;
      while (j < messages.length && messages[j].role === 'assistant') {
        consecutive.push(messages[j]);
        j++;
      }

      const primary = consecutive[consecutive.length - 1];
      const earlierVersions: MessageVersion[] = consecutive.slice(0, -1).map((m) => ({
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

/**
 * agentRunSlice 保存跨路由存活的会话与 Agent 运行状态，按 conversationId 键控。
 * 组件卸载/路由切换时数据仍然保留，回到会话时直接续用，避免状态丢失与中间加载态。
 */
interface AgentRunSliceState {
  // conversationId -> 该会话当前活跃 run 的 Agent 运行状态
  runStates: Record<string, AgentRunViewState>;
  // conversationId -> 消息列表（已合并连续的助手消息）
  messages: Record<string, Message[]>;
  // conversationId -> 该会话最新 run_id
  conversationRunIds: Record<string, string>;
  // conversationId -> runId -> 历史运行状态（版本切换用）
  historicalRunStates: Record<string, Record<string, AgentRunViewState>>;
  // conversationId -> 是否已完成历史运行状态回放
  historicalHydratedIds: Record<string, boolean>;
  // conversationId -> 已完成完整回放的活跃 run_id（终态 run 再次进入时直接跳过，避免中间加载态）
  activeHydratedRunIds: Record<string, string>;
}

const initialState: AgentRunSliceState = {
  runStates: {},
  messages: {},
  conversationRunIds: {},
  historicalRunStates: {},
  historicalHydratedIds: {},
  activeHydratedRunIds: {},
};

const agentRunSlice = createSlice({
  name: 'agentRun',
  initialState,
  reducers: {
    /** 将 Agent 运行 action（事件或生命周期动作）应用到指定会话的运行状态 */
    applyRunAction(state, action: PayloadAction<{ conversationId: string; runAction: AgentRunAction }>) {
      const { conversationId, runAction } = action.payload;
      if (!conversationId) return;
      const current = state.runStates[conversationId] ?? initialAgentRunState;
      state.runStates[conversationId] = reduceAgentEvent(current, runAction);
    },
    /** 整体覆盖指定会话的消息列表（首次从 API 加载时使用） */
    setMessages(state, action: PayloadAction<{ conversationId: string; messages: Message[] }>) {
      const { conversationId, messages } = action.payload;
      if (!conversationId) return;
      state.messages[conversationId] = mergeConsecutiveAssistantMessages(messages);
    },
    /** 追加一条用户消息 */
    appendUserMessage(state, action: PayloadAction<{ conversationId: string; message: Message }>) {
      const { conversationId, message } = action.payload;
      if (!conversationId) return;
      const current = state.messages[conversationId] ?? [];
      state.messages[conversationId] = mergeConsecutiveAssistantMessages([...current, message]);
    },
    /** 追加一条助手消息；带 replaceMessageId 时替换旧消息（重试场景），否则按 run_id 去重后追加 */
    appendAssistantMessage(state, action: PayloadAction<{ conversationId: string; message: Message; replaceMessageId?: string }>) {
      const { conversationId, message, replaceMessageId } = action.payload;
      if (!conversationId) return;
      const current = state.messages[conversationId] ?? [];

      if (replaceMessageId) {
        const idx = current.findIndex((m) => m.id === replaceMessageId);
        if (idx !== -1) {
          const old = current[idx];
          const oldVersion: MessageVersion = {
            content: old.content,
            agent_run_id: old.agent_run_id || '',
            status: old.status,
            created_at: old.created_at,
          };
          const replaced: Message = {
            ...message,
            versions: [...(old.versions || []), oldVersion],
            current_version_index: -1,
          };
          const next = [...current];
          next[idx] = replaced;
          state.messages[conversationId] = mergeConsecutiveAssistantMessages(next);
          return;
        }
        state.messages[conversationId] = mergeConsecutiveAssistantMessages([...current, message]);
        return;
      }

      // 去重：同一 run 的助手消息只保留一份
      if (current.some((m) => m.agent_run_id === message.agent_run_id && m.role === 'assistant')) return;
      state.messages[conversationId] = mergeConsecutiveAssistantMessages([...current, message]);
    },
    /** 切换消息版本（重试历史版本查看） */
    switchMessageVersion(state, action: PayloadAction<{ conversationId: string; messageId: string; versionIdx: number }>) {
      const { conversationId, messageId, versionIdx } = action.payload;
      if (!conversationId) return;
      const current = state.messages[conversationId] ?? [];
      state.messages[conversationId] = current.map((m) =>
        m.id === messageId ? { ...m, current_version_index: versionIdx } : m,
      );
    },
    /** 记录会话最新的 run_id */
    setConversationRunId(state, action: PayloadAction<{ conversationId: string; runId: string }>) {
      const { conversationId, runId } = action.payload;
      if (!conversationId) return;
      state.conversationRunIds[conversationId] = runId;
    },
    /** 缓存某会话历史 run 的运行状态 */
    setHistoricalRunState(state, action: PayloadAction<{ conversationId: string; runId: string; runState: AgentRunViewState }>) {
      const { conversationId, runId, runState } = action.payload;
      if (!conversationId) return;
      if (!state.historicalRunStates[conversationId]) state.historicalRunStates[conversationId] = {};
      state.historicalRunStates[conversationId][runId] = runState;
    },
    /** 标记会话的历史运行状态已完成回放 */
    markHistoricalHydrated(state, action: PayloadAction<string>) {
      state.historicalHydratedIds[action.payload] = true;
    },
    /** 标记会话的活跃 run 已完成完整回放（终态） */
    markActiveRunHydrated(state, action: PayloadAction<{ conversationId: string; runId: string }>) {
      const { conversationId, runId } = action.payload;
      if (!conversationId) return;
      state.activeHydratedRunIds[conversationId] = runId;
    },
    /** 删除会话时清理其全部状态（仅在删除会话时调用） */
    clearConversation(state, action: PayloadAction<string>) {
      const conversationId = action.payload;
      delete state.runStates[conversationId];
      delete state.messages[conversationId];
      delete state.conversationRunIds[conversationId];
      delete state.historicalRunStates[conversationId];
      delete state.historicalHydratedIds[conversationId];
      delete state.activeHydratedRunIds[conversationId];
    },
  },
});

export const {
  applyRunAction,
  setMessages,
  appendUserMessage,
  appendAssistantMessage,
  switchMessageVersion,
  setConversationRunId,
  setHistoricalRunState,
  markHistoricalHydrated,
  markActiveRunHydrated,
  clearConversation,
} = agentRunSlice.actions;
export default agentRunSlice.reducer;
