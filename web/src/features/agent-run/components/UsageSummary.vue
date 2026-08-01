<script setup lang="ts">
defineProps<{
  usage: {
    input_tokens: number
    output_tokens: number
    total_tokens: number
  }
  memoryCount: number
}>()
</script>

<template>
  <div class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-3">
    <h4 class="mb-2 text-xs font-medium text-[var(--memora-muted)]">
      用量
    </h4>

    <div class="grid grid-cols-2 gap-2 text-xs">
      <div>
        <span class="text-[var(--memora-muted)]">输入 Token:</span>
        <span class="ml-1 text-[var(--memora-text)]">{{ usage.input_tokens }}</span>
      </div>
      <div>
        <span class="text-[var(--memora-muted)]">输出 Token:</span>
        <span class="ml-1 text-[var(--memora-text)]">{{ usage.output_tokens }}</span>
      </div>
      <div>
        <span class="text-[var(--memora-muted)]">总计 Token:</span>
        <span class="ml-1 text-[var(--memora-text)]">{{ usage.total_tokens }}</span>
      </div>
      <div v-if="memoryCount > 0">
        <span class="text-[var(--memora-muted)]">记忆召回:</span>
        <span class="ml-1 text-[var(--memora-text)]">{{ memoryCount }}</span>
      </div>
    </div>
  </div>
</template>
