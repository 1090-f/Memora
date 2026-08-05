<script setup lang="ts">
import { ref, computed } from 'vue'
import { useDocumentScopeChat } from '../composables/useDocumentScopeChat'
import { useCreateConversation } from '@/features/conversation/queries'
import { useAuthStore } from '@/stores/auth'
import { useAgentRuntimeStore } from '@/stores/agent-runtime'
import { streamAgentEvents } from '@/api/sse'
import { request } from '@/api/client'
import { renderMarkdown, processExternalLinks } from '@/utils/markdown'
import type { Message } from '@/features/conversation/types'

const props = defineProps<{
  documentTitle?: string
  documentId?: string
  knowledgeBaseId?: string
}>()

const { getDocumentScopeLabel, buildQuestionPayload } = useDocumentScopeChat()
const authStore = useAuthStore()
const runtimeStore = useAgentRuntimeStore()

const scopeLabel = computed(() => getDocumentScopeLabel(props.documentTitle))

// Chat state
const input = ref('')
const isStreaming = ref(false)
const conversationId = ref<string | null>(null)
const messages = ref<Message[]>([])
let abortController: AbortController | null = null

const createConversationMutation = useCreateConversation(
  computed(() => props.knowledgeBaseId || ''),
)

function openFullChat() {
  if (props.knowledgeBaseId) {
    void window.open(`/chat/${props.knowledgeBaseId}`, '_blank')
  }
}

async function handleSubmit() {
  const query = input.value.trim()
  if (!query || isStreaming.value || !props.knowledgeBaseId) return

  input.value = ''

  // Create conversation if needed
  if (!conversationId.value) {
    try {
      const conv = await createConversationMutation.mutateAsync({
        title: `[文档] ${props.documentTitle || query.slice(0, 30)}`,
      })
      conversationId.value = conv.id
    } catch {
      return
    }
  }

  // Add user message
  const userMessage: Message = {
    id: `doc-user-${Date.now()}`,
    role: 'user',
    content: query,
    agent_run_id: null,
    created_at: new Date().toISOString(),
  }
  messages.value.push(userMessage)

  // Add assistant placeholder
  const assistantMessage: Message = {
    id: `doc-assistant-${Date.now()}`,
    role: 'assistant',
    content: '',
    agent_run_id: null,
    status: 'streaming',
    created_at: new Date().toISOString(),
  }
  messages.value.push(assistantMessage)

  isStreaming.value = true
  runtimeStore.reset()

  try {
    const payload = buildQuestionPayload(query, props.documentId)
    const result = await request<{ run_id: string; events_url: string; status: string }>(
      `/conversations/${conversationId.value}/questions`,
      { method: 'POST', body: payload },
    )

    assistantMessage.agent_run_id = result.run_id
    runtimeStore.startRun(result.run_id)

    // Start SSE
    if (!authStore.access_token) return
    abortController = new AbortController()

    await streamAgentEvents({
      url: result.events_url,
      access_token: authStore.access_token,
      signal: abortController.signal,
      on_event: (event) => {
        runtimeStore.handleEvent(event)
        if (runtimeStore.answer) {
          assistantMessage.content = runtimeStore.answer
        }
        if (runtimeStore.status === 'completed') {
          assistantMessage.status = 'completed'
          assistantMessage.citations = runtimeStore.citations
          isStreaming.value = false
        } else if (runtimeStore.status === 'failed') {
          assistantMessage.status = 'failed'
          isStreaming.value = false
        }
      },
    })
  } catch {
    assistantMessage.status = 'failed'
    isStreaming.value = false
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
    e.preventDefault()
    void handleSubmit()
  }
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="border-b border-[var(--memora-border)] px-4 py-3">
      <h3 class="text-sm font-medium text-[var(--memora-text)]">
        AI 助手
      </h3>
      <p class="mt-1 text-xs text-[var(--memora-muted)]">
        {{ scopeLabel }}
      </p>
    </div>

    <!-- Messages -->
    <div class="flex-1 overflow-y-auto p-3">
      <div
        v-if="messages.length === 0"
        class="flex h-full flex-col items-center justify-center"
      >
        <svg
          class="mb-3 h-10 w-10 text-[var(--memora-muted)]"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
        </svg>
        <p class="text-xs text-[var(--memora-muted)]">
          基于当前文档提问
        </p>
      </div>

      <div
        v-else
        class="space-y-3"
      >
        <div
          v-for="msg in messages"
          :key="msg.id"
          :class="[
            'rounded-lg px-3 py-2 text-sm',
            msg.role === 'user'
              ? 'bg-[var(--memora-brand-500)] text-white ml-8'
              : 'bg-[var(--memora-surface)] text-[var(--memora-text)] mr-8',
          ]"
        >
          <div v-if="msg.role === 'assistant' && msg.content">
            <!-- eslint-disable-next-line vue/no-v-html -->
            <div class="prose prose-xs max-w-none" v-html="processExternalLinks(renderMarkdown(msg.content))" />
          </div>
          <div v-else-if="msg.role === 'assistant' && msg.status === 'streaming'">
            <div class="flex items-center gap-2 text-[var(--memora-muted)]">
              <div class="h-2 w-2 animate-pulse rounded-full bg-[var(--memora-brand-500)]" />
              思考中...
            </div>
          </div>
          <div v-else>
            {{ msg.content }}
          </div>
        </div>
      </div>
    </div>

    <!-- Input -->
    <div class="border-t border-[var(--memora-border)] p-3">
      <div class="flex items-end gap-2">
        <textarea
          v-model="input"
          :disabled="isStreaming"
          placeholder="输入问题... (Ctrl+Enter)"
          rows="1"
          class="flex-1 resize-none rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-1.5 text-sm outline-none focus:border-[var(--memora-brand-500)] disabled:opacity-50"
          @keydown="handleKeydown"
        />
        <button
          :disabled="!input.trim() || isStreaming"
          class="rounded-md bg-[var(--memora-brand-500)] px-3 py-1.5 text-sm font-medium text-white hover:bg-[var(--memora-brand-600)] disabled:opacity-50"
          @click="handleSubmit"
        >
          {{ isStreaming ? '...' : '发送' }}
        </button>
      </div>

      <!-- Open full chat link -->
      <button
        v-if="knowledgeBaseId"
        class="mt-2 text-xs text-[var(--memora-muted)] hover:text-[var(--memora-text)]"
        @click="openFullChat"
      >
        打开完整聊天 →
      </button>
    </div>
  </div>
</template>
