<script setup lang="ts">
import { ref } from 'vue'
import type { DirectoryNode } from '../types'

defineProps<{
  directories: DirectoryNode[]
  selectedId?: string | null
}>()

const emit = defineEmits<{
  (e: 'select', id: string): void
  (e: 'selectDocument', id: string): void
}>()

const expandedIds = ref<Set<string>>(new Set())

function toggleExpand(id: string) {
  if (expandedIds.value.has(id)) {
    expandedIds.value.delete(id)
  } else {
    expandedIds.value.add(id)
  }
}

function isExpanded(id: string): boolean {
  return expandedIds.value.has(id)
}

function hasChildren(node: DirectoryNode): boolean {
  return node.children && node.children.length > 0
}
</script>

<template>
  <div class="py-2">
    <div
      v-for="node in directories"
      :key="node.id"
    >
      <!-- Directory item -->
      <div
        :class="[
          'flex items-center gap-1 px-2 py-1 text-sm cursor-pointer hover:bg-[var(--memora-bg)] rounded-md',
          selectedId === node.id && 'bg-[var(--memora-brand-500)]/10 text-[var(--memora-brand-500)]',
        ]"
        :style="{ paddingLeft: `${(node.depth - 1) * 16 + 8}px` }"
        @click="toggleExpand(node.id)"
      >
        <svg
          v-if="hasChildren(node)"
          class="h-4 w-4 flex-shrink-0 text-[var(--memora-muted)] transition-transform"
          :class="{ 'rotate-90': isExpanded(node.id) }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
        </svg>
        <span
          v-else
          class="w-4"
        />

        <svg
          class="h-4 w-4 flex-shrink-0 text-[var(--memora-muted)]"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
        </svg>

        <span class="truncate">{{ node.name }}</span>
      </div>

      <!-- Children -->
      <div v-if="hasChildren(node) && isExpanded(node.id)">
        <KnowledgeTree
          :directories="node.children"
          :selected-id="selectedId"
          @select="emit('select', $event)"
          @select-document="emit('selectDocument', $event)"
        />
      </div>
    </div>
  </div>
</template>
