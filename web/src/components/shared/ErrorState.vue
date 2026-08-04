<script setup lang="ts">
import { computed } from 'vue'
import { AppError } from '@/api/errors'

const props = defineProps<{
  error: Error | null
  title?: string
}>()

const isRetryable = computed(() => {
  if (!props.error) return false
  if (props.error instanceof AppError) {
    return props.error.httpStatus >= 500 || props.error.code === 'RATE_LIMITED'
  }
  return true
})

const requestId = computed(() => {
  if (props.error instanceof AppError) {
    return props.error.requestId
  }
  return null
})

const errorMessage = computed(() => {
  if (props.error instanceof AppError) {
    return props.error.message
  }
  return props.error?.message || '发生未知错误'
})

defineEmits<{
  (e: 'retry'): void
}>()

function copyRequestId() {
  if (requestId.value) {
    void navigator.clipboard.writeText(requestId.value)
  }
}
</script>

<template>
  <div class="flex flex-col items-center justify-center py-12">
    <svg
      class="mb-4 h-12 w-12 text-[var(--memora-danger)]"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
    </svg>

    <h3 class="mb-2 text-sm font-medium text-[var(--memora-text)]">
      {{ title || '加载失败' }}
    </h3>

    <p class="mb-4 max-w-md text-center text-sm text-[var(--memora-muted)]">
      {{ errorMessage }}
    </p>

    <div class="flex items-center gap-3">
      <button
        v-if="isRetryable"
        class="rounded-md bg-[var(--memora-brand-500)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--memora-brand-600)]"
        @click="$emit('retry')"
      >
        重试
      </button>

      <button
        v-if="requestId"
        class="rounded-md border border-[var(--memora-border)] px-4 py-2 text-sm text-[var(--memora-muted)] hover:bg-[var(--memora-bg)]"
        @click="copyRequestId"
      >
        复制请求 ID
      </button>
    </div>

    <p
      v-if="requestId"
      class="mt-2 text-xs text-[var(--memora-muted)]"
    >
      Request ID: {{ requestId }}
    </p>
  </div>
</template>
