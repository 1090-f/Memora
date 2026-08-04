<script setup lang="ts">
defineProps<{
  title?: string
  description?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center py-12 text-center">
    <svg
      class="mb-4 h-12 w-12 text-[var(--memora-muted)]"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="1.5"
        d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
      />
    </svg>
    <h3 class="mb-1 text-sm font-medium text-[var(--memora-text)]">
      {{ title || '暂无数据' }}
    </h3>
    <p
      v-if="description"
      class="text-sm text-[var(--memora-muted)]"
    >
      {{ description }}
    </p>
    <div v-if="$slots.action" class="mt-4">
      <slot name="action" />
    </div>
  </div>
</template>
