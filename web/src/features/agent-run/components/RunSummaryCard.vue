<script setup lang="ts">
import type { AgentRun } from '../api'
import StatusBadge from '@/components/shared/StatusBadge.vue'

defineProps<{
  run: AgentRun
}>()

defineEmits<{
  (e: 'select', id: string): void
}>()

const statusVariant = (status: string) => {
  switch (status) {
    case 'completed': return 'success'
    case 'failed': return 'danger'
    case 'running': return 'info'
    case 'cancelled': return 'warning'
    default: return 'default'
  }
}

function formatDuration(ms: number | null): string {
  if (!ms) return '-'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}
</script>

<template>
  <div
    class="cursor-pointer rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4 transition-shadow hover:shadow-md"
    @click="$emit('select', run.id)"
  >
    <div class="mb-2 flex items-start justify-between">
      <div class="flex items-center gap-2">
        <StatusBadge
          :status="run.status"
          :variant="statusVariant(run.status)"
        />
        <span
          v-if="run.execution_mode"
          class="text-xs text-[var(--memora-muted)]"
        >
          {{ run.execution_mode === 'react' ? 'ReAct' : 'Plan-Execute' }}
        </span>
      </div>
      <span class="text-xs text-[var(--memora-muted)]">
        {{ formatDuration(run.duration_ms) }}
      </span>
    </div>

    <p class="mb-2 text-sm text-[var(--memora-text)] line-clamp-2">
      {{ run.query }}
    </p>

    <div class="flex items-center justify-between text-xs text-[var(--memora-muted)]">
      <span>{{ run.total_tokens ? `${run.total_tokens} tokens` : '-' }}</span>
      <span>{{ new Date(run.created_at).toLocaleString() }}</span>
    </div>
  </div>
</template>
