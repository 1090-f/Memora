<script setup lang="ts">
import BaseDrawer from '@/components/base/BaseDrawer.vue'
import StatusBadge from '@/components/shared/StatusBadge.vue'
import type { Memory } from '../types'

defineProps<{
  open: boolean
  memory: Memory | null
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'activate', id: string): void
  (e: 'deactivate', id: string): void
  (e: 'delete', id: string): void
}>()

const typeLabels: Record<string, string> = {
  preference: '偏好',
  project: '项目',
  decision: '决策',
  goal: '目标',
  fact: '事实',
  progress: '进展',
}

const scopeLabels: Record<string, string> = {
  user: '用户',
  knowledge_base: '知识库',
}
</script>

<template>
  <BaseDrawer
    :open="open"
    title="Memory 详情"
    :width="480"
    @update:open="emit('update:open', $event)"
  >
    <div v-if="memory" class="space-y-4">
      <div>
        <label class="mb-1 block text-xs text-[var(--memora-muted)]">类型</label>
        <p class="text-sm text-[var(--memora-text)]">
          {{ typeLabels[memory.memory_type] || memory.memory_type }}
        </p>
      </div>

      <div>
        <label class="mb-1 block text-xs text-[var(--memora-muted)]">作用域</label>
        <p class="text-sm text-[var(--memora-text)]">
          {{ scopeLabels[memory.scope_type] || memory.scope_type }}
        </p>
      </div>

      <div>
        <label class="mb-1 block text-xs text-[var(--memora-muted)]">状态</label>
        <StatusBadge :status="memory.status" />
      </div>

      <div v-if="memory.summary">
        <label class="mb-1 block text-xs text-[var(--memora-muted)]">摘要</label>
        <p class="text-sm text-[var(--memora-text)]">
          {{ memory.summary }}
        </p>
      </div>

      <div>
        <label class="mb-1 block text-xs text-[var(--memora-muted)]">内容</label>
        <p class="text-sm text-[var(--memora-text)] whitespace-pre-wrap">
          {{ memory.content }}
        </p>
      </div>

      <div v-if="memory.importance !== null">
        <label class="mb-1 block text-xs text-[var(--memora-muted)]">重要性</label>
        <p class="text-sm text-[var(--memora-text)]">
          {{ (memory.importance * 100).toFixed(0) }}%
        </p>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-xs text-[var(--memora-muted)]">创建时间</label>
          <p class="text-sm text-[var(--memora-text)]">
            {{ new Date(memory.created_at).toLocaleString() }}
          </p>
        </div>
        <div>
          <label class="mb-1 block text-xs text-[var(--memora-muted)]">更新时间</label>
          <p class="text-sm text-[var(--memora-text)]">
            {{ new Date(memory.updated_at).toLocaleString() }}
          </p>
        </div>
      </div>

      <div v-if="memory.last_accessed_at">
        <label class="mb-1 block text-xs text-[var(--memora-muted)]">最近使用</label>
        <p class="text-sm text-[var(--memora-text)]">
          {{ new Date(memory.last_accessed_at).toLocaleString() }}
        </p>
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-2 pt-4 border-t border-[var(--memora-border)]">
        <button
          v-if="memory.status === 'active'"
          class="rounded-md px-3 py-1.5 text-sm text-[var(--memora-muted)] hover:bg-[var(--memora-bg)]"
          @click="emit('deactivate', memory.id)"
        >
          停用
        </button>
        <button
          v-else-if="memory.status === 'inactive'"
          class="rounded-md px-3 py-1.5 text-sm text-[var(--memora-brand-500)] hover:bg-[var(--memora-brand-500)]/10"
          @click="emit('activate', memory.id)"
        >
          启用
        </button>
        <button
          class="rounded-md px-3 py-1.5 text-sm text-[var(--memora-danger)] hover:bg-red-50"
          @click="emit('delete', memory.id)"
        >
          删除
        </button>
      </div>
    </div>
  </BaseDrawer>
</template>
