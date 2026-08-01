<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessageList, useSubmitQuestion, useCreateConversation } from '../queries'
import { useWorkspaceStore } from '@/stores/workspace'
import ChatWorkspaceShell from '@/layouts/ChatWorkspaceShell.vue'
import ConversationSidebar from '../components/ConversationSidebar.vue'
import MessageList from '../components/MessageList.vue'
import ChatComposer from '../components/ChatComposer.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import type { Message } from '../types'

const route = useRoute()
const router = useRouter()
const workspaceStore = useWorkspaceStore()

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
const pendingRunId = ref<string | null>(null)
const isStreaming = ref(false)

// Watch for server messages and merge with local
watch(messagesData, (data) => {
  if (data?.items) {
    // Merge server messages with local optimistic ones
    const serverIds = new Set(data.items.map(m => m.id))
    const optimisticOnly = localMessages.value.filter(m => !serverIds.has(m.id))
    localMessages.value = [...data.items, ...optimisticOnly]
  }
}, { immediate: true })

const messages = computed(() => localMessages.value)

// Mutations
const submitMutation = useSubmitQuestion(computed(() => conversationId.value || ''))
const createConversationMutation = useCreateConversation(kbId)

async function handleSubmit(query: string) {
  if (!query.trim()) return

  // If no conversation, create one first
  if (!conversationId.value) {
    try {
      const conv = await createConversationMutation.mutateAsync({ title: query.slice(0, 50) })
      void router.push(`/chat/${kbId.value}/${conv.id}`)
      // Will submit after navigation
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

  try {
    const result = await submitMutation.mutateAsync({ query })
    pendingRunId.value = result.run_id

    // Update assistant message with run_id
    assistantMessage.agent_run_id = result.run_id

    // Task 8 will handle SSE streaming
    // For now, mark as completed after a delay
    setTimeout(() => {
      assistantMessage.status = 'completed'
      isStreaming.value = false
    }, 2000)
  } catch {
    assistantMessage.status = 'failed'
    isStreaming.value = false
  }
}

function handleStop() {
  // Task 8 will implement stop functionality
  isStreaming.value = false
}

function handleSelectConversation(id: string) {
  void router.push(`/chat/${kbId.value}/${id}`)
}
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
        <EmptyState
          title="提交问题后显示执行过程"
          description="Agent 运行状态和工具调用将在此处展示"
        />
      </div>
    </template>
  </ChatWorkspaceShell>
</template>
