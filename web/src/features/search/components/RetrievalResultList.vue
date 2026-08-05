<script setup lang="ts">
import type { SearchResult } from '../types'

defineProps<{
  results: SearchResult[]
  title?: string
}>()
</script>

<template>
  <div class="space-y-3">
    <h3
      v-if="title"
      class="text-sm font-medium text-[var(--memora-text)]"
    >
      {{ title }}
    </h3>

    <div
      v-for="(result, index) in results"
      :key="result.chunk_id"
      class="rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] p-3"
    >
      <div class="mb-1 flex items-center justify-between">
        <span class="text-sm font-medium text-[var(--memora-text)]">
          {{ index + 1 }}. {{ result.document_title }}
        </span>
        <span class="text-xs text-[var(--memora-muted)]">
          排名: {{ result.final_rank }}
        </span>
      </div>
      <p class="text-sm text-[var(--memora-text)] line-clamp-2">
        {{ result.content }}
      </p>
    </div>
  </div>
</template>
