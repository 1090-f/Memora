<script setup lang="ts">
import { computed } from 'vue'
import { useMcpToolList, useToggleMcpTool } from '../queries'
import EmptyState from '@/components/shared/EmptyState.vue'
import LoadingSkeleton from '@/components/shared/LoadingSkeleton.vue'
import type { McpTool } from '../types'

const props = defineProps<{
  serverId: string
}>()

const serverIdComputed = computed(() => props.serverId)
const { data: tools, isLoading } = useMcpToolList(serverIdComputed)
const toggleMutation = useToggleMcpTool(serverIdComputed)

async function handleToggle(tool: McpTool) {
  if (!tool.read_only) return // Don't allow enabling write tools
  await toggleMutation.mutateAsync({ toolId: tool.id, enabled: !tool.enabled })
}

function handleViewSchema(tool: McpTool) {
  // Placeholder for schema drawer - could be implemented as a modal
  alert(JSON.stringify(tool.input_schema, null, 2))
}
</script>

<template>
  <div class="space-y-3">
    <h3 class="text-sm font-medium text-[var(--memora-text)]">
      工具列表
    </h3>

    <LoadingSkeleton
      v-if="isLoading"
      type="list"
      :rows="3"
    />

    <EmptyState
      v-else-if="!tools?.length"
      title="暂无工具"
      description="点击发现工具按钮扫描 MCP Server"
    />

    <div
      v-else
      class="space-y-2"
    >
      <div
        v-for="tool in tools"
        :key="tool.id"
        class="flex items-center justify-between rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] p-3"
      >
        <div class="flex items-center gap-3">
          <div>
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-[var(--memora-text)]">
                {{ tool.name }}
              </span>
              <span
                v-if="tool.read_only"
                class="rounded bg-green-100 px-1.5 py-0.5 text-xs text-green-700"
              >
                只读
              </span>
              <span
                v-else
                class="rounded bg-red-100 px-1.5 py-0.5 text-xs text-red-700"
              >
                写入
              </span>
            </div>
            <p
              v-if="tool.description"
              class="mt-0.5 text-xs text-[var(--memora-muted)] line-clamp-1"
            >
              {{ tool.description }}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <button
            class="rounded px-2 py-1 text-xs text-[var(--memora-muted)] hover:bg-[var(--memora-bg)]"
            @click="handleViewSchema(tool)"
          >
            Schema
          </button>
          <button
            :class="[
              'relative inline-flex h-5 w-9 items-center rounded-full transition-colors',
              tool.enabled ? 'bg-[var(--memora-brand-500)]' : 'bg-gray-300',
              !tool.read_only && 'opacity-50 cursor-not-allowed',
            ]"
            :disabled="!tool.read_only"
            :aria-label="tool.enabled ? '禁用' : '启用'"
            @click="handleToggle(tool)"
          >
            <span
              :class="[
                'inline-block h-3 w-3 transform rounded-full bg-white transition-transform',
                tool.enabled ? 'translate-x-5' : 'translate-x-1',
              ]"
            />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
