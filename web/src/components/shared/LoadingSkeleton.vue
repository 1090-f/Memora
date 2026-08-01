<script setup lang="ts">
withDefaults(defineProps<{
  rows?: number
  type?: 'text' | 'card' | 'list'
}>(), {
  rows: 3,
  type: 'text',
})
</script>

<template>
  <div v-if="type === 'card'" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
    <div
      v-for="i in rows"
      :key="i"
      class="animate-pulse rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
    >
      <div class="mb-3 h-4 w-3/4 rounded bg-gray-200" />
      <div class="mb-2 h-3 w-full rounded bg-gray-200" />
      <div class="h-3 w-2/3 rounded bg-gray-200" />
    </div>
  </div>

  <div
    v-else-if="type === 'list'"
    class="space-y-3"
  >
    <div
      v-for="i in rows"
      :key="i"
      class="animate-pulse flex items-center gap-3 rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-3"
    >
      <div class="h-10 w-10 flex-shrink-0 rounded bg-gray-200" />
      <div class="flex-1">
        <div class="mb-1 h-3 w-1/3 rounded bg-gray-200" />
        <div class="h-3 w-2/3 rounded bg-gray-200" />
      </div>
    </div>
  </div>

  <div
    v-else
    class="space-y-3"
  >
    <div
      v-for="i in rows"
      :key="i"
      class="animate-pulse h-4 rounded bg-gray-200"
      :style="{ width: `${70 + Math.random() * 30}%` }"
    />
  </div>
</template>
