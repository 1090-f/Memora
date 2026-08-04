<script setup lang="ts">
import type { ProcessingState } from '../types'

defineProps<{
  state: ProcessingState | null
  loading?: boolean
}>()

defineEmits<{
  (e: 'retry'): void
}>()

const statusLabels: Record<string, string> = {
  pending: '等待处理',
  parsing: '解析中',
  cleaning: '清洗中',
  chunking: '分块中',
  embedding: '向量化中',
  keyword_indexing: '关键词索引中',
  succeeded: '处理完成',
  failed: '处理失败',
}

function isTerminal(status: string): boolean {
  return status === 'succeeded' || status === 'failed'
}
</script>

<template>
  <div
    v-if="loading"
    class="animate-pulse space-y-2 p-4"
  >
    <div class="h-4 w-1/3 rounded bg-gray-200" />
    <div class="h-3 w-1/2 rounded bg-gray-200" />
  </div>

  <div
    v-else-if="state"
    class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
  >
    <div class="mb-2 flex items-center gap-2">
      <span
        :class="[
          'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
          state.processing_status === 'succeeded' && 'bg-green-100 text-green-800',
          state.processing_status === 'failed' && 'bg-red-100 text-red-800',
          !isTerminal(state.processing_status) && 'bg-blue-100 text-blue-800',
        ]"
      >
        {{ statusLabels[state.processing_status] || state.processing_status }}
      </span>

      <button
        v-if="state.processing_status === 'failed'"
        class="rounded-md px-2 py-1 text-xs font-medium text-[var(--memora-brand-500)] hover:bg-[var(--memora-brand-500)]/10"
        @click="$emit('retry')"
      >
        重试
      </button>
    </div>

    <div
      v-if="state.failure_reason"
      class="text-sm text-[var(--memora-danger)]"
    >
      {{ state.failure_reason }}
    </div>

    <div class="mt-2 text-xs text-[var(--memora-muted)]">
      当前索引版本: {{ state.current_index_version }} | 活跃版本: {{ state.active_index_version }}
    </div>
  </div>
</template>
