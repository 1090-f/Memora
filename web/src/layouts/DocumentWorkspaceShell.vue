<script setup lang="ts">
import { ref } from 'vue'
import { useLayoutStore } from '@/stores/layout'

const layoutStore = useLayoutStore()

const isResizing = ref(false)
const resizeStartX = ref(0)
const resizeStartWidth = ref(0)

function startResize(e: MouseEvent) {
  isResizing.value = true
  resizeStartX.value = e.clientX
  resizeStartWidth.value = layoutStore.document_sidebar_width

  function handleMouseMove(e: MouseEvent) {
    const delta = e.clientX - resizeStartX.value
    layoutStore.setDocumentSidebarWidth(resizeStartWidth.value + delta)
  }

  function handleMouseUp() {
    isResizing.value = false
    document.removeEventListener('mousemove', handleMouseMove)
    document.removeEventListener('mouseup', handleMouseUp)
  }

  document.addEventListener('mousemove', handleMouseMove)
  document.addEventListener('mouseup', handleMouseUp)
}
</script>

<template>
  <div class="flex h-full overflow-hidden">
    <!-- Left sidebar: Tree -->
    <aside
      class="flex flex-col border-r border-[var(--memora-border)] bg-[var(--memora-surface)]"
      :style="{ width: `${layoutStore.document_sidebar_width}px` }"
    >
      <slot name="sidebar" />
    </aside>

    <!-- Resize handle -->
    <div
      class="w-1 cursor-col-resize bg-[var(--memora-border)] hover:bg-[var(--memora-brand-500)]"
      :class="{ 'bg-[var(--memora-brand-500)]': isResizing }"
      @mousedown="startResize"
    />

    <!-- Center: Document viewer -->
    <main class="flex flex-1 flex-col overflow-hidden bg-[var(--memora-surface)]">
      <slot name="toolbar" />
      <slot name="viewer" />
    </main>

    <!-- Right: AI Panel -->
    <aside
      v-if="!layoutStore.inspector_collapsed"
      class="flex flex-col border-l border-[var(--memora-border)] bg-[var(--memora-surface)]"
      :style="{ width: `${layoutStore.inspector_width}px` }"
    >
      <slot name="inspector" />
    </aside>
  </div>
</template>
