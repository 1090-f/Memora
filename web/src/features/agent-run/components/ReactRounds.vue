<script setup lang="ts">
import type { RuntimeRound } from '@/stores/agent-runtime'

defineProps<{
  rounds: RuntimeRound[]
}>()
</script>

<template>
  <div
    v-if="rounds.length > 0"
    class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-3"
  >
    <h4 class="mb-2 text-xs font-medium text-[var(--memora-muted)]">
      ReAct 轮次
    </h4>

    <div class="space-y-1">
      <div
        v-for="round in rounds"
        :key="round.round_no"
        class="flex items-center gap-2 text-xs"
      >
        <span class="flex-shrink-0 text-[var(--memora-muted)]">
          #{{ round.round_no }}
        </span>
        <span class="text-[var(--memora-text)]">
          {{ round.action_summary || '处理中...' }}
        </span>
      </div>
    </div>
  </div>
</template>
