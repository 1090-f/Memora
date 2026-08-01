<script setup lang="ts">
defineProps<{
  mode: 'react' | 'plan_execute' | null
  reason: string
}>()
</script>

<template>
  <div
    v-if="mode"
    class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-3"
  >
    <div class="mb-1 flex items-center gap-2">
      <span class="text-xs font-medium text-[var(--memora-muted)]">Router</span>
      <span
        :class="[
          'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
          mode === 'react' ? 'bg-blue-100 text-blue-800' : 'bg-purple-100 text-purple-800',
        ]"
      >
        {{ mode === 'react' ? 'ReAct' : 'Plan-Execute' }}
      </span>
    </div>
    <p class="text-xs text-[var(--memora-text)]">
      {{ reason }}
    </p>
  </div>
</template>
