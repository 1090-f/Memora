<script setup lang="ts">
import { computed } from 'vue'
import { renderMarkdown, processExternalLinks } from '@/utils/markdown'
import type { DocumentDetail, ProcessingStatus } from '../types'

const props = defineProps<{
  document: DocumentDetail | null
  processingStatus?: ProcessingStatus
}>()

const renderedContent = computed(() => {
  if (!props.document?.content) return ''
  return processExternalLinks(renderMarkdown(props.document.content))
})

const isEmpty = computed(() => {
  return props.document?.processing_status === 'succeeded' && !props.document?.content
})
</script>

<template>
  <div class="flex-1 overflow-y-auto px-8 py-6">
    <!-- Loading state -->
    <div
      v-if="!document"
      class="flex items-center justify-center py-12"
    >
      <div class="text-sm text-[var(--memora-muted)]">
        加载中...
      </div>
    </div>

    <!-- Processing states -->
    <div
      v-else-if="processingStatus && processingStatus !== 'succeeded' && processingStatus !== 'failed'"
      class="flex flex-col items-center justify-center py-12"
    >
      <div class="mb-4 h-8 w-8 animate-spin rounded-full border-2 border-[var(--memora-brand-500)] border-t-transparent" />
      <p class="text-sm text-[var(--memora-muted)]">
        文档处理中: {{ processingStatus }}...
      </p>
    </div>

    <!-- Empty content -->
    <div
      v-else-if="isEmpty"
      class="flex flex-col items-center justify-center py-12"
    >
      <svg
        class="mb-4 h-12 w-12 text-[var(--memora-muted)]"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
      <p class="text-sm text-[var(--memora-muted)]">
        文档内容为空
      </p>
    </div>

    <!-- Rendered content -->
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div
      v-else
      class="prose prose-sm max-w-[800px] text-[var(--memora-text)]"
      v-html="renderedContent"
    />
  </div>
</template>
