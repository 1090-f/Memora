<script setup lang="ts">
import { ref } from 'vue'

withDefaults(defineProps<{
  content: string
  side?: 'top' | 'bottom' | 'left' | 'right'
}>(), {
  side: 'top',
})

const isVisible = ref(false)
</script>

<template>
  <div
    class="relative inline-block"
    @mouseenter="isVisible = true"
    @mouseleave="isVisible = false"
  >
    <slot />

    <div
      v-if="isVisible && content"
      :class="[
        'absolute z-50 whitespace-nowrap rounded-md bg-[var(--memora-nav)] px-2 py-1 text-xs text-white shadow-md',
        side === 'top' && 'bottom-full left-1/2 mb-2 -translate-x-1/2',
        side === 'bottom' && 'top-full left-1/2 mt-2 -translate-x-1/2',
        side === 'left' && 'bottom-1/2 right-full mr-2 translate-y-1/2',
        side === 'right' && 'bottom-1/2 left-full ml-2 translate-y-1/2',
      ]"
      role="tooltip"
    >
      {{ content }}
    </div>
  </div>
</template>
