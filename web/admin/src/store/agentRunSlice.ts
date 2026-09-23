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
 * 从 runAction 中提取 runId：
 * - AgentEvent 自带 run_id；
 * - HYDRATE_AGENT_RUN_STATE 从 run.id 取；
 * - RESET / SET_QUEUED / SET_CANCELLED 等生命周期 action 无 run_id，需调用方显式传入 runId。
 */
function extractRunId(runAction: AgentRunAction, explicitRunId?: string): string | undefined {
  if ('run_id' in runAction && typeof runAction.run_id === 'string') return runAction.run_id;
  if (runAction.type === 'HYDRATE_AGENT_RUN_STATE') return runAction.run?.id;
  return explicitRunId;
}

/**
 * agentRunSlice 保存跨路由存活的会话与 Agent 运行状态，按 conversationId 键控。
 * 组件卸载/路由切换时数据仍然保留，回到会话时直接续用，避免状态丢失与中间加载态。
 */
interface AgentRunSliceState {
  // conversationId -> 该会话当前活跃 run 的 Agent 运行状态（用于续传、暂停按钮、流式回答）
  runStates: Record<string, AgentRunViewState>;
  // conversationId -> 消息列表（已合并连续的助手消息）
  messages: Record<string, Message[]>;
  // conversationId -> 该会话最新 run_id
  conversationRunIds: Record<string, string>;
  // conversationId -> runId -> 该 run 的完整运行状态（实时事件累积 + 历史回放写入），
  // 用于让每一条 AI 回复都能展示其对应的运行过程。
  runStatesByRunId: Record<string, Record<string, AgentRunViewState>>;
  // conversationId -> runId -> 是否已完成历史回放（按 runId 幂等，替代会话级布尔闸门）
  hydratedRunIds: Record<string, Record<string, boolean>>;
  // conversationId -> runId -> 是否正在回放（防重复请求 + 渲染骨架用）
  pendingRunIds: Record<string, Record<string, boolean>>;
  // conversationId -> 已完成完整回放的活跃 run_id（终态 run 再次进入时直接跳过，避免中间加载态）
  activeHydratedRunIds: Record<string, string>;
}

const initialState: AgentRunSliceState = {
  runStates: {},
  messages: {},
  conversationRunIds: {},
  runStatesByRunId: {},
  hydratedRunIds: {},
  pendingRunIds: {},
  activeHydratedRunIds: {},
};

const agentRunSlice = createSlice({
  name: 'agentRun',
  initialState,
  reducers: {
    /** 将 Agent 运行 action（事件或生命周期动作）应用到指定会话的运行状态，并按 runId 同步存档 */
    applyRunAction(
      state,
      action: PayloadAction<{ conversationId: string; runId?: string; runAction: AgentRunAction }>,
    ) {
      const { conversationId, runId: explicitRunId, runAction } = action.payload;
      if (!conversationId) return;
      const current = state.runStates[conversationId] ?? initialAgentRunState;
      const next = reduceAgentEvent(current, runAction);
      state.runStates[conversationId] = next;

      // 按 runId 存档：每次事件都同步写入，run 终态后该存档即为完整过程，本会话内零请求即可展示。
      const runId = extractRunId(runAction, explicitRunId);
      if (runId && next.status !== 'idle') {
        if (!state.runStatesByRunId[conversationId]) state.runStatesByRunId[conversationId] = {};
        state.runStatesByRunId[conversationId][runId] = next;
      }
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

      // 去重：同一 run 的助手消息只保留一份。
      // 例外：已存在的那条是空内容（竞态下先写入的空占位），而新消息有实质内容时，
      // 用新消息覆盖它 —— 否则空消息会永久占位，后来送到嘴边的正确回答被丢弃且无法自愈。
      const existingIdx = current.findIndex((m) => m.agent_run_id === message.agent_run_id && m.role === 'assistant');
      if (existingIdx !== -1) {
        const existing = current[existingIdx];
        const existingIsEmpty = !existing.content || existing.content.trim() === '';
        const incomingHasContent = Boolean(message.content && message.content.trim() !== '');
        if (!(existingIsEmpty && incomingHasContent)) return;
        const next = [...current];
        // 保留原 id：引用跳转 / 版本切换 / 运行详情入口都按 id 定位，换 id 会让它们失效。
        next[existingIdx] = { ...existing, ...message, id: existing.id };
        state.messages[conversationId] = mergeConsecutiveAssistantMessages(next);
        return;
      }
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
    /** 按 runId 写入某个 run 的完整运行状态（历史回放完成后使用） */
    setRunStateByRunId(state, action: PayloadAction<{ conversationId: string; runId: string; runState: AgentRunViewState }>) {
      const { conversationId, runId, runState } = action.payload;
      if (!conversationId || !runId) return;
      if (!state.runStatesByRunId[conversationId]) state.runStatesByRunId[conversationId] = {};
      state.runStatesByRunId[conversationId][runId] = runState;
    },
    /** 标记某个 run 已完成历史回放（按 runId 幂等） */
    markRunHydrated(state, action: PayloadAction<{ conversationId: string; runId: string }>) {
      const { conversationId, runId } = action.payload;
      if (!conversationId || !runId) return;
      if (!state.hydratedRunIds[conversationId]) state.hydratedRunIds[conversationId] = {};
      state.hydratedRunIds[conversationId][runId] = true;
    },
    /** 标记某个 run 正在回放 */
    markRunPending(state, action: PayloadAction<{ conversationId: string; runId: string }>) {
      const { conversationId, runId } = action.payload;
      if (!conversationId || !runId) return;
      if (!state.pendingRunIds[conversationId]) state.pendingRunIds[conversationId] = {};
      state.pendingRunIds[conversationId][runId] = true;
    },
    /** 清除某个 run 的回放中标记 */
    clearRunPending(state, action: PayloadAction<{ conversationId: string; runId: string }>) {
      const { conversationId, runId } = action.payload;
      if (!conversationId || !runId) return;
      const map = state.pendingRunIds[conversationId];
      if (map) delete map[runId];
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
      delete state.runStatesByRunId[conversationId];
      delete state.hydratedRunIds[conversationId];
      delete state.pendingRunIds[conversationId];
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
  setRunStateByRunId,
  markRunHydrated,
  markRunPending,
  clearRunPending,
  markActiveRunHydrated,
  clearConversation,
} = agentRunSlice.actions;
export default agentRunSlice.reducer;
