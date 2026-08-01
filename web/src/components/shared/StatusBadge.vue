<script setup lang="ts">
withDefaults(defineProps<{
  status: string
  variant?: 'default' | 'success' | 'warning' | 'danger' | 'info'
}>(), {
  variant: 'default',
})
</script>

<template>
  <span
    :class="[
      'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
      variant === 'success' && 'bg-green-100 text-green-800',
      variant === 'warning' && 'bg-yellow-100 text-yellow-800',
      variant === 'danger' && 'bg-red-100 text-red-800',
      variant === 'info' && 'bg-blue-100 text-blue-800',
      variant === 'default' && 'bg-gray-100 text-gray-800',
    ]"
  >
    {{ status }}
  </span>
</template>
