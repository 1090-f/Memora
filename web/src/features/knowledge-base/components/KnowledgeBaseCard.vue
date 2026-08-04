<script setup lang="ts">
import type { KnowledgeBase } from '../types'

defineProps<{
  knowledgeBase: KnowledgeBase
}>()

defineEmits<{
  (e: 'select', id: string): void
  (e: 'edit', id: string): void
  (e: 'delete', id: string): void
}>()
</script>

<template>
  <div
    class="group cursor-pointer rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4 transition-shadow hover:shadow-md"
    @click="$emit('select', knowledgeBase.id)"
  >
    <div class="mb-3 flex items-start justify-between">
      <div class="flex h-10 w-10 items-center justify-center rounded-lg bg-[var(--memora-brand-500)]/10 text-[var(--memora-brand-500)]">
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
        </svg>
      </div>

      <div class="flex gap-1 opacity-0 group-hover:opacity-100">
        <button
          class="rounded p-1 text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
          aria-label="编辑"
          @click.stop="$emit('edit', knowledgeBase.id)"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
          </svg>
        </button>
        <button
          class="rounded p-1 text-[var(--memora-muted)] hover:bg-red-50 hover:text-[var(--memora-danger)]"
          aria-label="删除"
          @click.stop="$emit('delete', knowledgeBase.id)"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </button>
      </div>
    </div>

    <h3 class="mb-1 text-sm font-medium text-[var(--memora-text)]">
      {{ knowledgeBase.name }}
    </h3>

    <p
      v-if="knowledgeBase.description"
      class="mb-3 line-clamp-2 text-xs text-[var(--memora-muted)]"
    >
      {{ knowledgeBase.description }}
    </p>

    <div class="flex items-center gap-3 text-xs text-[var(--memora-muted)]">
      <span>{{ knowledgeBase.document_count ?? 0 }} 篇文档</span>
      <span v-if="knowledgeBase.agent_enabled" class="text-[var(--memora-success)]">Agent 已启用</span>
    </div>
  </div>
</template>
