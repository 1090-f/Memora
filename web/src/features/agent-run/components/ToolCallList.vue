<script setup lang="ts">
import type { RuntimeToolCall } from '@/stores/agent-runtime'

defineProps<{
  toolCalls: RuntimeToolCall[]
}>()
</script>

<template>
  <div
    v-if="toolCalls.length > 0"
    class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-3"
  >
    <h4 class="mb-2 text-xs font-medium text-[var(--memora-muted)]">
      工具调用
    </h4>

    <div class="space-y-2">
      <div
        v-for="tool in toolCalls"
        :key="tool.tool_call_id"
        class="rounded-md bg-[var(--memora-bg)] p-2"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <span
              :class="[
                'h-2 w-2 rounded-full',
                tool.status === 'succeeded' && 'bg-green-500',
                tool.status === 'running' && 'bg-blue-500 animate-pulse',
                tool.status === 'failed' && 'bg-red-500',
              ]"
            />
            <span class="text-xs font-medium text-[var(--memora-text)]">
              {{ tool.tool_name }}
            </span>
            <span class="text-xs text-[var(--memora-muted)]">
              ({{ tool.tool_type }})
            </span>
          </div>
          <span
            v-if="tool.duration_ms"
            class="text-xs text-[var(--memora-muted)]"
          >
            {{ tool.duration_ms }}ms
          </span>
        </div>
        <p
          v-if="tool.input_summary"
          class="mt-1 text-xs text-[var(--memora-muted)] line-clamp-1"
        >
          {{ tool.input_summary }}
        </p>
        <p
          v-if="tool.error_message"
          class="mt-1 text-xs text-[var(--memora-danger)]"
        >
          {{ tool.error_message }}
        </p>
      </div>
    </div>
  </div>
</template>
