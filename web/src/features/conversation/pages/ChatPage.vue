<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessageList, useSubmitQuestion, useCreateConversation } from '../queries'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAuthStore } from '@/stores/auth'
import { useAgentRuntimeStore } from '@/stores/agent-runtime'
import { streamAgentEvents } from '@/api/sse'
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

// Set current knowledge base
watch(kbId, (id) => {
  if (id) workspaceStore.setCurrentKbId(id)
}, { immediate: true })

// Conversation and messages
const { data: messagesData, isLoading: messagesLoading } = useMessageList(conversationId)

// Local messages state for optimistic updates
const localMessages = ref<Message[]>([])
const isStreaming = ref(false)
let abortController: AbortController | null = null

// Watch for server messages and merge with local
watch(messagesData, (data) => {
  if (data?.items) {
    const serverIds = new Set(data.items.map(m => m.id))
    const optimisticOnly = localMessages.value.filter(m => !serverIds.has(m.id))
    localMessages.value = [...data.items, ...optimisticOnly]
  }
}, { immediate: true })

const messages = computed(() => localMessages.value)

// Mutations
const submitMutation = useSubmitQuestion(computed(() => conversationId.value || ''))
const createConversationMutation = useCreateConversation(kbId)

async function startSseStream(eventsUrl: string, assistantMessage: Message) {
  if (!authStore.access_token) return

  abortController = new AbortController()

  try {
    await streamAgentEvents({
      url: eventsUrl,
      access_token: authStore.access_token,
      signal: abortController.signal,
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
      assistantMessage.status = 'failed'
      isStreaming.value = false
    }
  }
}

async function handleSubmit(query: string) {
  if (!query.trim()) return

  // If no conversation, create one first
  if (!conversationId.value) {
    try {
      const conv = await createConversationMutation.mutateAsync({ title: query.slice(0, 50) })
      void router.push(`/chat/${kbId.value}/${conv.id}`)
      return
    } catch {
      return
    }
  }

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
    const result = await submitMutation.mutateAsync({ query })
    assistantMessage.agent_run_id = result.run_id
    runtimeStore.startRun(result.run_id)

    // Start SSE stream
    void startSseStream(result.events_url, assistantMessage)
  } catch {
    assistantMessage.status = 'failed'
    isStreaming.value = false
  }
}

function handleStop() {
  if (abortController) {
    abortController.abort()
    abortController = null
  }

  // Cancel via API if we have a run_id
  if (runtimeStore.run_id && authStore.access_token) {
    void fetch(`/api/v1/agent-runs/${runtimeStore.run_id}/cancel`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${authStore.access_token}` },
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
      <MessageList
        :messages="messages"
        :loading="messagesLoading"
      />
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
