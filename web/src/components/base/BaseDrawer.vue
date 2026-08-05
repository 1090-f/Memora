<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'

const props = withDefaults(defineProps<{
  open: boolean
  title?: string
  side?: 'left' | 'right'
  width?: number
}>(), {
  title: '',
  side: 'right',
  width: 400,
})

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'close'): void
}>()

const drawerRef = ref<HTMLDivElement | null>(null)

function handleClose() {
  emit('update:open', false)
  emit('close')
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.open) {
    handleClose()
  }
}

function handleBackdropClick() {
  handleClose()
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    document.body.style.overflow = 'hidden'
  } else {
    document.body.style.overflow = ''
  }
})
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex">
      <!-- Backdrop -->
      <div
        class="absolute inset-0 bg-black/50"
        @click="handleBackdropClick"
      />

      <!-- Drawer Panel -->
      <div
        ref="drawerRef"
        :class="[
          'relative flex flex-col bg-[var(--memora-surface)] shadow-xl',
          side === 'right' && 'ml-auto',
        ]"
        :style="{ width: `${width}px` }"
      >
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-[var(--memora-border)] px-6 py-4">
          <h2 class="text-lg font-semibold text-[var(--memora-text)]">
            {{ title }}
          </h2>
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

        <!-- Body -->
        <div class="flex-1 overflow-y-auto px-6 py-4">
          <slot />
        </div>

        <!-- Footer -->
        <div
          v-if="$slots.footer"
          class="flex items-center justify-end gap-2 border-t border-[var(--memora-border)] px-6 py-4"
        >
          <slot name="footer" />
        </div>
      </div>
    </div>
  </Teleport>
</template>
