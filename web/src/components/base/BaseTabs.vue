<script setup lang="ts">
defineProps<{
  tabs: Array<{ key: string; label: string }>
  modelValue: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()
</script>

<template>
  <div class="flex border-b border-[var(--memora-border)]">
    <button
      v-for="tab in tabs"
      :key="tab.key"
      :class="[
        'px-4 py-2 text-sm font-medium transition-colors',
        modelValue === tab.key
          ? 'border-b-2 border-[var(--memora-brand-500)] text-[var(--memora-brand-500)]'
          : 'text-[var(--memora-muted)] hover:text-[var(--memora-text)]',
      ]"
      @click="emit('update:modelValue', tab.key)"
    >
      {{ tab.label }}
    </button>
  </div>
</template>
