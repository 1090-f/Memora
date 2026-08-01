<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  open: boolean
  title?: string
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'close'): void
}>()

const dialogRef = ref<HTMLDialogElement | null>(null)

watch(() => props.open, (isOpen) => {
  if (!dialogRef.value) return
  if (isOpen) {
    dialogRef.value.showModal()
  } else {
    dialogRef.value.close()
  }
})

function handleClose() {
  emit('update:open', false)
  emit('close')
}

function handleBackdropClick(e: MouseEvent) {
  if (e.target === dialogRef.value) {
    handleClose()
  }
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    handleClose()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <dialog
    ref="dialogRef"
    class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-0 shadow-lg backdrop:bg-black/50"
    @click="handleBackdropClick"
  >
    <div
      v-if="title || $slots.header"
      class="flex items-center justify-between border-b border-[var(--memora-border)] px-6 py-4"
    >
      <slot name="header">
        <h2 class="text-lg font-semibold text-[var(--memora-text)]">
          {{ title }}
        </h2>
      </slot>
      <button
        class="rounded-md p-1 text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
        aria-label="关闭"
        @click="handleClose"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
    <div class="px-6 py-4">
      <slot />
    </div>
    <div
      v-if="$slots.footer"
      class="flex items-center justify-end gap-2 border-t border-[var(--memora-border)] px-6 py-4"
    >
      <slot name="footer" />
    </div>
  </dialog>
</template>
