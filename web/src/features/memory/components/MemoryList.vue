<script setup lang="ts">
import type { Memory } from '../types'
import StatusBadge from '@/components/shared/StatusBadge.vue'

defineProps<{
  memories: Memory[]
}>()

defineEmits<{
  (e: 'select', id: string): void
  (e: 'activate', id: string): void
  (e: 'deactivate', id: string): void
  (e: 'delete', id: string): void
}>()

const typeLabels: Record<string, string> = {
  preference: '偏好',
  project: '项目',
  decision: '决策',
  goal: '目标',
  fact: '事实',
  progress: '进展',
}

const statusVariant = (status: string) => {
  switch (status) {
    case 'active': return 'success'
    case 'inactive': return 'default'
    default: return 'default'
  }
}
</script>

<template>
  <div class="space-y-3">
    <div
      v-for="memory in memories"
      :key="memory.id"
      class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
    >
      <div class="mb-2 flex items-start justify-between">
        <div class="flex items-center gap-2">
          <span class="text-xs font-medium text-[var(--memora-muted)]">
            {{ typeLabels[memory.memory_type] || memory.memory_type }}
          </span>
          <StatusBadge
            :status="memory.status"
            :variant="statusVariant(memory.status)"
          />
        </div>
        <div class="flex items-center gap-1">
          <button
            v-if="memory.status === 'active'"
            class="rounded px-2 py-1 text-xs text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
            @click="$emit('deactivate', memory.id)"
          >
            停用
          </button>
          <button
            v-else-if="memory.status === 'inactive'"
            class="rounded px-2 py-1 text-xs text-[var(--memora-brand-500)] hover:bg-[var(--memora-brand-500)]/10"
            @click="$emit('activate', memory.id)"
          >
            启用
          </button>
          <button
            class="rounded px-2 py-1 text-xs text-[var(--memora-danger)] hover:bg-red-50"
            @click="$emit('delete', memory.id)"
          >
            删除
          </button>
        </div>
      </div>

      <p
        v-if="memory.summary"
        class="mb-1 text-sm font-medium text-[var(--memora-text)]"
      >
        {{ memory.summary }}
      </p>
      <p class="text-sm text-[var(--memora-text)] line-clamp-3">
        {{ memory.content }}
      </p>

      <div class="mt-2 flex items-center gap-4 text-xs text-[var(--memora-muted)]">
        <span v-if="memory.source_conversation_id">
          来源会话
        </span>
        <span>
          {{ new Date(memory.created_at).toLocaleString() }}
        </span>
        <span v-if="memory.importance !== null">
          重要性: {{ (memory.importance * 100).toFixed(0) }}%
        </span>
      </div>
    </div>
  </div>
</template>
