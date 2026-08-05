<script setup lang="ts">
import type { McpServer } from '../types'
import StatusBadge from '@/components/shared/StatusBadge.vue'

defineProps<{
  server: McpServer
}>()

defineEmits<{
  (e: 'edit', id: string): void
  (e: 'delete', id: string): void
  (e: 'test', id: string): void
  (e: 'discover', id: string): void
  (e: 'toggle', id: string, enabled: boolean): void
}>()

const statusVariant = (status: string) => {
  switch (status) {
    case 'available': return 'success'
    case 'unavailable': return 'danger'
    case 'testing': return 'info'
    default: return 'default'
  }
}
</script>

<template>
  <div class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4">
    <div class="mb-3 flex items-start justify-between">
      <div>
        <div class="flex items-center gap-2">
          <h3 class="text-sm font-medium text-[var(--memora-text)]">
            {{ server.name }}
          </h3>
          <StatusBadge
            :status="server.status"
            :variant="statusVariant(server.status)"
          />
        </div>
        <p class="mt-1 text-xs text-[var(--memora-muted)]">
          {{ server.url }}
        </p>
      </div>
      <div class="flex items-center gap-1">
        <button
          class="rounded p-1 text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
          aria-label="测试连接"
          @click="$emit('test', server.id)"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </button>
        <button
          class="rounded p-1 text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
          aria-label="发现工具"
          @click="$emit('discover', server.id)"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </button>
        <button
          class="rounded p-1 text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
          aria-label="编辑"
          @click="$emit('edit', server.id)"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
          </svg>
        </button>
        <button
          class="rounded p-1 text-[var(--memora-muted)] hover:bg-red-50 hover:text-[var(--memora-danger)]"
          aria-label="删除"
          @click="$emit('delete', server.id)"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
        </button>
      </div>
    </div>

    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2 text-xs text-[var(--memora-muted)]">
        <span v-if="server.auth_configured">已配置认证</span>
        <span v-if="server.last_tested_at">
          上次测试: {{ new Date(server.last_tested_at).toLocaleString() }}
        </span>
      </div>
      <button
        :class="[
          'relative inline-flex h-5 w-9 items-center rounded-full transition-colors',
          server.enabled ? 'bg-[var(--memora-brand-500)]' : 'bg-gray-300',
        ]"
        :aria-label="server.enabled ? '禁用' : '启用'"
        @click="$emit('toggle', server.id, !server.enabled)"
      >
        <span
          :class="[
            'inline-block h-3 w-3 transform rounded-full bg-white transition-transform',
            server.enabled ? 'translate-x-5' : 'translate-x-1',
          ]"
        />
      </button>
    </div>
  </div>
</template>
