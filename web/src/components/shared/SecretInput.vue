<script setup lang="ts">
import { ref, computed } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: string
  configured?: boolean
  mode?: 'create' | 'replace'
  placeholder?: string
}>(), {
  configured: false,
  mode: 'create',
  placeholder: '',
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const showSecret = ref(false)

const displayValue = computed(() => {
  if (props.configured && !showSecret.value && props.mode === 'replace') {
    return '••••••••'
  }
  return props.modelValue
})

function handleInput(e: Event) {
  const target = e.target as HTMLInputElement
  emit('update:modelValue', target.value)
}
</script>

<template>
  <div class="relative">
    <input
      :type="showSecret ? 'text' : 'password'"
      :value="displayValue"
      :placeholder="configured && mode === 'replace' ? '输入新密钥替换已有密钥' : placeholder"
      :disabled="configured && mode === 'replace' && !showSecret"
      class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 pr-10 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)] disabled:opacity-50"
      @input="handleInput"
    >
    <button
      type="button"
      class="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-[var(--memora-muted)] hover:text-[var(--memora-text)]"
      :aria-label="showSecret ? '隐藏密钥' : '显示密钥'"
      @click="showSecret = !showSecret"
    >
      <svg
        v-if="showSecret"
        class="h-4 w-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
      </svg>
      <svg
        v-else
        class="h-4 w-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
      </svg>
    </button>
    <p
      v-if="configured && mode === 'replace'"
      class="mt-1 text-xs text-[var(--memora-muted)]"
    >
      已配置密钥。点击眼睛图标输入新密钥以替换。
    </p>
  </div>
</template>
