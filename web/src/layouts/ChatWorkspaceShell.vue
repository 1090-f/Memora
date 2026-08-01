<script setup lang="ts">
import { useLayoutStore } from '@/stores/layout'

const layoutStore = useLayoutStore()
</script>

<template>
  <div class="flex h-full overflow-hidden">
    <!-- Left sidebar: Conversations -->
    <aside
      class="flex flex-col border-r border-[var(--memora-border)] bg-[var(--memora-surface)]"
      :style="{ width: `${layoutStore.document_sidebar_width}px` }"
    >
      <slot name="sidebar" />
    </aside>

    <!-- Center: Messages -->
    <main class="flex flex-1 flex-col overflow-hidden bg-[var(--memora-bg)]">
      <slot name="messages" />
      <slot name="composer" />
    </main>

    <!-- Right: Inspector -->
    <aside
      v-if="!layoutStore.inspector_collapsed"
      class="flex flex-col border-l border-[var(--memora-border)] bg-[var(--memora-surface)]"
      :style="{ width: `${layoutStore.inspector_width}px` }"
    >
      <slot name="inspector" />
    </aside>
  </div>
</template>
