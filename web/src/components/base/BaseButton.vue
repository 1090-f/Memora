<script setup lang="ts">
defineProps<{
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
  size?: 'sm' | 'md' | 'lg'
  disabled?: boolean
  loading?: boolean
}>()
</script>

<template>
  <button
    :disabled="disabled || loading"
    :class="[
      'inline-flex items-center justify-center rounded-md font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-[var(--memora-brand-500)] focus:ring-offset-2 disabled:opacity-50',
      variant === 'primary' && 'bg-[var(--memora-brand-500)] text-white hover:bg-[var(--memora-brand-600)]',
      variant === 'secondary' && 'border border-[var(--memora-border)] bg-[var(--memora-surface)] text-[var(--memora-text)] hover:bg-[var(--memora-bg)]',
      variant === 'danger' && 'bg-[var(--memora-danger)] text-white hover:bg-red-700',
      variant === 'ghost' && 'text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]',
      !variant && 'bg-[var(--memora-brand-500)] text-white hover:bg-[var(--memora-brand-600)]',
      size === 'sm' && 'px-3 py-1.5 text-xs',
      size === 'lg' && 'px-6 py-3 text-base',
      (!size || size === 'md') && 'px-4 py-2 text-sm',
    ]"
  >
    <span
      v-if="loading"
      class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"
    />
    <slot />
  </button>
</template>
