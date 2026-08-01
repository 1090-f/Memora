<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{
  initialStatus?: string
  initialMode?: string
}>()

const emit = defineEmits<{
  (e: 'change', filters: { status?: string; execution_mode?: string }): void
}>()

const status = ref(props.initialStatus || '')
const mode = ref(props.initialMode || '')

watch([status, mode], () => {
  emit('change', {
    status: status.value || undefined,
    execution_mode: mode.value || undefined,
  })
})

function clear() {
  status.value = ''
  mode.value = ''
}
</script>

<template>
  <div class="flex items-center gap-4">
    <select
      v-model="status"
      class="rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-1.5 text-sm outline-none focus:border-[var(--memora-brand-500)]"
    >
      <option value="">全部状态</option>
      <option value="running">执行中</option>
      <option value="completed">已完成</option>
      <option value="failed">失败</option>
      <option value="cancelled">已取消</option>
    </select>

    <select
      v-model="mode"
      class="rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-1.5 text-sm outline-none focus:border-[var(--memora-brand-500)]"
    >
      <option value="">全部模式</option>
      <option value="react">ReAct</option>
      <option value="plan_execute">Plan-Execute</option>
    </select>

    <button
      v-if="status || mode"
      class="text-xs text-[var(--memora-muted)] hover:text-[var(--memora-text)]"
      @click="clear"
    >
      清除筛选
    </button>
  </div>
</template>
