<script setup lang="ts">
import type { Message } from '../types'

defineProps<{
  message: Message
}>()
</script>

<template>
  <div class="flex justify-end">
    <div class="max-w-[70%] rounded-lg bg-[var(--memora-brand-500)] px-4 py-2 text-sm text-white">
      {{ message.content }}
    </div>
  </div>
</template>
