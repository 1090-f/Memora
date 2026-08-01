<script setup lang="ts">
import type { DocumentDetail } from '../types'

defineProps<{
  document: DocumentDetail | null
}>()

defineEmits<{
  (e: 'delete'): void
  (e: 'reindex'): void
}>()
</script>

<template>
  <div
    v-if="document"
    class="flex items-center justify-between border-b border-[var(--memora-border)] px-6 py-3"
  >
    <!-- Breadcrumb-like title -->
    <div class="flex items-center gap-2">
      <h1 class="text-lg font-semibold text-[var(--memora-text)]">
        {{ document.title }}
      </h1>
      <span
        :class="[
          'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
          document.processing_status === 'succeeded' && 'bg-green-100 text-green-800',
          document.processing_status === 'failed' && 'bg-red-100 text-red-800',
          document.processing_status === 'pending' && 'bg-yellow-100 text-yellow-800',
          document.processing_status === 'parsing' && 'bg-blue-100 text-blue-800',
          document.processing_status === 'cleaning' && 'bg-blue-100 text-blue-800',
          document.processing_status === 'chunking' && 'bg-blue-100 text-blue-800',
          document.processing_status === 'embedding' && 'bg-blue-100 text-blue-800',
          document.processing_status === 'keyword_indexing' && 'bg-blue-100 text-blue-800',
        ]"
      >
        {{ document.processing_status }}
      </span>
    </div>

    <!-- Actions -->
    <div class="flex items-center gap-2">
      <button
        v-if="document.processing_status === 'failed'"
        class="rounded-md px-3 py-1.5 text-xs font-medium text-[var(--memora-brand-500)] hover:bg-[var(--memora-brand-500)]/10"
        @click="$emit('reindex')"
      >
        重试
      </button>
      <button
        v-if="document.processing_status === 'succeeded'"
        class="rounded-md px-3 py-1.5 text-xs font-medium text-[var(--memora-muted)] hover:bg-[var(--memora-bg)]"
        @click="$emit('reindex')"
      >
        重新索引
      </button>
      <button
        class="rounded-md px-3 py-1.5 text-xs font-medium text-[var(--memora-danger)] hover:bg-red-50"
        @click="$emit('delete')"
      >
        删除
      </button>
    </div>
  </div>
</template>
