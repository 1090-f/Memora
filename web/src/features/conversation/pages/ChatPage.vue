<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessageList, useCreateConversation } from '../queries'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAuthStore } from '@/stores/auth'
import { useAgentRuntimeStore } from '@/stores/agent-runtime'
import { streamAgentEvents } from '@/api/sse'
import { request } from '@/api/client'
import type { AgentRun } from '@/features/agent-run/api'
import ChatWorkspaceShell from '@/layouts/ChatWorkspaceShell.vue'
import ConversationSidebar from '../components/ConversationSidebar.vue'
import MessageList from '../components/MessageList.vue'
import ChatComposer from '../components/ChatComposer.vue'
import AgentRunPanel from '@/features/agent-run/components/AgentRunPanel.vue'
import type { Message } from '../types'

const route = useRoute()
const router = useRouter()
const workspaceStore = useWorkspaceStore()
const authStore = useAuthStore()
const runtimeStore = useAgentRuntimeStore()

const kbId = computed(() => route.params.kbId as string)
const conversationId = computed(() => route.params.conversationId as string | undefined)

const sseResumeEnabled = import.meta.env.VITE_SSE_RESUME_ENABLED === 'true'

// Set current knowledge base
watch(kbId, (id) => {
  if (id) workspaceStore.setCurrentKbId(id)
}, { immediate: true })

// Conversation and messages
const { data: messagesData, isLoading: messagesLoading, refetch: refetchMessages } = useMessageList(conversationId)

// Local messages state for optimistic updates
const localMessages = ref<Message[]>([])
const isStreaming = ref(false)
const showReconnect = ref(false)
let abortController: AbortController | null = null

// Pending question for redirect scenario
const pendingQuery = ref<string | null>(null)

// Watch for server messages and merge with local
watch(messagesData, (data) => {
  if (data?.items) {
    const serverIds = new Set(data.items.map(m => m.id))
    const optimisticOnly = localMessages.value.filter(m => !serverIds.has(m.id))
    localMessages.value = [...data.items, ...optimisticOnly]
  }
}, { immediate: true })

// After navigation to new conversation, submit the pending question
watch(conversationId, (newId, oldId) => {
  if (newId && !oldId && pendingQuery.value) {
    const query = pendingQuery.value
    pendingQuery.value = null
    // Small delay to ensure component is mounted
    setTimeout(() => {
      void submitQuestionDirect(newId, query)
    }, 100)
  }
})

const messages = computed(() => localMessages.value)

// Mutations
const createConversationMutation = useCreateConversation(kbId)

async function startSseStream(eventsUrl: string, assistantMessage: Message, afterSequence?: number) {
  if (!authStore.access_token) return

  abortController = new AbortController()
  showReconnect.value = false

  try {
    await streamAgentEvents({
      url: eventsUrl,
      access_token: authStore.access_token,
      signal: abortController.signal,
      after_sequence: afterSequence,
      on_event: (event) => {
        runtimeStore.handleEvent(event)

        // Update assistant message content from runtime answer
        if (runtimeStore.answer) {
          assistantMessage.content = runtimeStore.answer
        }

        // Update status on completion
        if (runtimeStore.status === 'completed') {
          assistantMessage.status = 'completed'
          assistantMessage.citations = runtimeStore.citations
          isStreaming.value = false
          // Refresh persisted messages from server
          void refetchMessages()
        } else if (runtimeStore.status === 'failed') {
          assistantMessage.status = 'failed'
          isStreaming.value = false
        } else if (runtimeStore.status === 'cancelled') {
          assistantMessage.status = 'cancelled'
          isStreaming.value = false
        }
      },
    })
  } catch (err) {
    if ((err as Error).name !== 'AbortError') {
      // SSE disconnected - check run status
      void handleSseDisconnect(assistantMessage)
    }
  }
}

async function handleSseDisconnect(assistantMessage: Message) {
  if (!runtimeStore.run_id) {
    assistantMessage.status = 'failed'
    isStreaming.value = false
    return
  }

  try {
    const run = await request<AgentRun>(`/agent-runs/${runtimeStore.run_id}`)

    if (run.status === 'completed') {
      // Run completed while disconnected - refresh messages
      assistantMessage.status = 'completed'
      assistantMessage.content = run.final_result || assistantMessage.content
      isStreaming.value = false
      void refetchMessages()
    } else if (run.status === 'failed' || run.status === 'cancelled') {
      assistantMessage.status = run.status
      isStreaming.value = false
    } else if (run.status === 'running' || run.status === 'queued') {
      // Still running - show reconnect option
      isStreaming.value = false
      showReconnect.value = true
    }
  } catch {
    assistantMessage.status = 'failed'
    isStreaming.value = false
  }
}

function handleReconnect() {
  if (!runtimeStore.run_id || !authStore.access_token) return

  // Find the streaming assistant message
  const assistantMessage = localMessages.value.find(
    m => m.agent_run_id === runtimeStore.run_id && m.role === 'assistant',
  )
  if (!assistantMessage) return

  isStreaming.value = true
  showReconnect.value = false

  const eventsUrl = `/api/v1/agent-runs/${runtimeStore.run_id}/events`
  const afterSeq = sseResumeEnabled ? runtimeStore.last_sequence + 1 : undefined
  void startSseStream(eventsUrl, assistantMessage, afterSeq)
}

async function submitQuestionDirect(convId: string, query: string) {
  // Add user message optimistically
  const userMessage: Message = {
    id: `temp-${Date.now()}`,
    role: 'user',
    content: query,
    agent_run_id: null,
    created_at: new Date().toISOString(),
  }
  localMessages.value.push(userMessage)

  // Add placeholder assistant message
  const assistantMessage: Message = {
    id: `temp-assistant-${Date.now()}`,
    role: 'assistant',
    content: '',
    agent_run_id: null,
    status: 'streaming',
    created_at: new Date().toISOString(),
  }
  localMessages.value.push(assistantMessage)

  isStreaming.value = true
  runtimeStore.reset()

  try {
    // Use direct request since submitMutation is bound to conversationId
    const result = await request<{ run_id: string; events_url: string; status: string }>(
      `/conversations/${convId}/questions`,
      { method: 'POST', body: { query } },
    )
    assistantMessage.agent_run_id = result.run_id
    runtimeStore.startRun(result.run_id)
    void startSseStream(result.events_url, assistantMessage)
  } catch {
    assistantMessage.status = 'failed'
    isStreaming.value = false
  }
}

async function handleSubmit(query: string) {
  if (!query.trim()) return

  // If no conversation, create one first and store pending query
  if (!conversationId.value) {
    pendingQuery.value = query
    try {
      const conv = await createConversationMutation.mutateAsync({ title: query.slice(0, 50) })
      void router.push(`/chat/${kbId.value}/${conv.id}`)
      // The watch on conversationId will trigger submitQuestionDirect
      return
    } catch {
      pendingQuery.value = null
      return
    }
  }

  await submitQuestionDirect(conversationId.value, query)
}

function handleStop() {
  if (abortController) {
    abortController.abort()
    abortController = null
  }

  // Cancel via API using unified client
  if (runtimeStore.run_id) {
    void request(`/agent-runs/${runtimeStore.run_id}/cancel`, {
      method: 'POST',
    }).catch(() => {})
  }

  isStreaming.value = false
}

function handleSelectConversation(id: string) {
  void router.push(`/chat/${kbId.value}/${id}`)
}

onUnmounted(() => {
  if (abortController) {
    abortController.abort()
  }
})
</script>

<template>
  <ChatWorkspaceShell>
    <template #sidebar>
      <ConversationSidebar
        :knowledge-base-id="kbId"
        :selected-id="conversationId || null"
        @select="handleSelectConversation"
      />
    </template>

    <template #messages>
      <div class="flex flex-1 flex-col overflow-hidden">
        <MessageList
          :messages="messages"
          :loading="messagesLoading"
        />

        <!-- Reconnect banner -->
        <div
          v-if="showReconnect"
          class="border-t border-[var(--memora-border)] bg-yellow-50 px-4 py-3 text-center"
        >
          <p class="mb-2 text-sm text-yellow-800">
            连接中断，任务仍在执行中
          </p>
          <button
            class="rounded-md bg-yellow-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-yellow-700"
            @click="handleReconnect"
          >
            重新连接
          </button>
        </div>
      </div>
    </template>

    <template #composer>
      <ChatComposer
        :disabled="!conversationId && createConversationMutation.isPending.value"
        :loading="isStreaming"
        @submit="handleSubmit"
        @stop="handleStop"
      />
    </template>

    <template #inspector>
      <div class="flex h-full flex-col">
        <div class="border-b border-[var(--memora-border)] px-4 py-3">
          <h3 class="text-sm font-medium text-[var(--memora-text)]">
            执行过程
          </h3>
        </div>
        <AgentRunPanel />
      </div>
    </template>
  </ChatWorkspaceShell>
</template>
