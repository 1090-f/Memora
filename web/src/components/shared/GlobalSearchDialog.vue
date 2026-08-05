<script setup lang="ts">
import { ref, computed } from 'vue'
import BaseDialog from '@/components/base/BaseDialog.vue'

defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'navigate', path: string): void
}>()

const searchQuery = ref('')

const navigationItems = [
  { label: '智能问答', path: '/chat', icon: 'chat' },
  { label: '文档工作区', path: '/kb', icon: 'document' },
  { label: 'Agent 运行记录', path: '/runs', icon: 'runs' },
  { label: '长期记忆', path: '/memories', icon: 'memory' },
  { label: 'MCP 配置', path: '/mcp', icon: 'mcp' },
  { label: '设置', path: '/settings', icon: 'settings' },
]

const filteredItems = computed(() => {
  if (!searchQuery.value) return navigationItems
  const query = searchQuery.value.toLowerCase()
  return navigationItems.filter(item =>
    item.label.toLowerCase().includes(query),
  )
})

function handleSelect(path: string) {
  emit('navigate', path)
  emit('update:open', false)
  searchQuery.value = ''
}
</script>

<template>
  <BaseDialog
    :open="open"
    @update:open="emit('update:open', $event)"
  >
    <div class="w-[400px]">
      <input
        v-model="searchQuery"
        type="text"
        placeholder="搜索页面..."
        class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)]"
        autofocus
      >

      <div class="mt-3 max-h-[300px] overflow-y-auto">
        <button
          v-for="item in filteredItems"
          :key="item.path"
          class="flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm text-[var(--memora-text)] hover:bg-[var(--memora-bg)]"
          @click="handleSelect(item.path)"
        >
          {{ item.label }}
        </button>

        <p
          v-if="filteredItems.length === 0"
          class="py-4 text-center text-sm text-[var(--memora-muted)]"
        >
          无匹配结果
        </p>
      </div>
    </div>
  </BaseDialog>
</template>
