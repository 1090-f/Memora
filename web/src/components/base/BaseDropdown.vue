<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

defineProps<{
  label?: string
}>()

const isOpen = ref(false)
const dropdownRef = ref<HTMLDivElement | null>(null)

function toggle() {
  isOpen.value = !isOpen.value
}

function close() {
  isOpen.value = false
}

function handleClickOutside(e: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(e.target as Node)) {
    close()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<template>
  <div ref="dropdownRef" class="relative inline-block">
    <button
      class="inline-flex items-center gap-1 rounded-md px-3 py-1.5 text-sm text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
      @click="toggle"
    >
      <slot name="trigger" />
      <svg
        class="h-4 w-4 transition-transform"
        :class="{ 'rotate-180': isOpen }"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <div
      v-if="isOpen"
      class="absolute right-0 z-50 mt-1 min-w-[160px] rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] py-1 shadow-lg"
    >
      <slot @close="close" />
    </div>
  </div>
</template>
