<script setup lang="ts">
import type { Citation } from '@/features/conversation/types'
import CitationPopover from './CitationPopover.vue'

defineProps<{
  citations: Citation[]
}>()
</script>

<template>
  <div
    v-if="citations.length > 0"
    class="mt-3 flex flex-wrap gap-2"
  >
    <CitationPopover
      v-for="(citation, index) in citations"
      :key="index"
      :citation="citation"
    />
  </div>
</template>
