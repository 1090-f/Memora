<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  documentTitle?: string
  knowledgeBaseId?: string
}>()

const documentScopeEnabled = computed(() => {
  return import.meta.env.VITE_DOCUMENT_SCOPE_ENABLED === 'true'
})

const scopeLabel = computed(() => {
  if (documentScopeEnabled.value && props.documentTitle) {
    return `基于当前文档: ${props.documentTitle}`
  }
  return '基于当前知识库'
})

function openFullChat() {
  if (props.knowledgeBaseId) {
    void window.open(`/chat/${props.knowledgeBaseId}`, '_blank')
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

    <!-- Content placeholder - Task 9 will connect to conversations -->
    <div class="flex flex-1 flex-col items-center justify-center p-4">
      <svg
        class="mb-3 h-10 w-10 text-[var(--memora-muted)]"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
      </svg>
      <p class="mb-3 text-center text-xs text-[var(--memora-muted)]">
        提交问题后显示 AI 回答
      </p>
      <button
        v-if="knowledgeBaseId"
        class="rounded-md px-3 py-1.5 text-xs font-medium text-[var(--memora-brand-500)] hover:bg-[var(--memora-brand-500)]/10"
        @click="openFullChat"
      >
        打开完整聊天
      </button>
    </div>
  </div>
</template>
