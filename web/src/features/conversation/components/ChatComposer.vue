<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{
  disabled?: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'submit', query: string): void
  (e: 'stop'): void
}>()

const input = ref('')

function handleSubmit() {
  const query = input.value.trim()
  if (!query || props.disabled) return
  emit('submit', query)
  input.value = ''
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
    e.preventDefault()
    handleSubmit()
  }
}
</script>

<template>
  <div class="border-t border-[var(--memora-border)] bg-[var(--memora-surface)] px-4 py-3">
    <div class="mx-auto max-w-[900px]">
      <div class="flex items-end gap-2">
        <textarea
          v-model="input"
          :disabled="disabled"
          placeholder="输入问题... (Ctrl+Enter 发送)"
          rows="1"
          class="flex-1 resize-none rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)] disabled:opacity-50"
          @keydown="handleKeydown"
        />
        <button
          v-if="loading"
          class="rounded-md bg-[var(--memora-danger)] px-4 py-2 text-sm font-medium text-white hover:bg-red-700"
          @click="emit('stop')"
        >
          停止
        </button>
        <button
          v-else
          :disabled="disabled || !input.trim()"
          class="rounded-md bg-[var(--memora-brand-500)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--memora-brand-600)] disabled:opacity-50"
          @click="handleSubmit"
        >
          发送
        </button>
      </div>
    </div>
  </div>
</template>
