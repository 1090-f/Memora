<script setup lang="ts">
import { computed } from 'vue'
import { useImportTaskList } from '../queries'
import StatusBadge from '@/components/shared/StatusBadge.vue'
import EmptyState from '@/components/shared/EmptyState.vue'

const props = defineProps<{
  knowledgeBaseId: string
}>()

const kbId = computed(() => props.knowledgeBaseId)
const { data: tasks, isLoading } = useImportTaskList(kbId)

const statusVariant = (status: string) => {
  switch (status) {
    case 'succeeded': return 'success'
    case 'failed': return 'danger'
    case 'running': return 'info'
    case 'pending': return 'warning'
    default: return 'default'
  }
}
</script>

<template>
  <div class="space-y-3">
    <h3 class="text-sm font-medium text-[var(--memora-text)]">
      导入任务
    </h3>

    <LoadingSkeleton
      v-if="isLoading"
      type="list"
      :rows="3"
    />

    <EmptyState
      v-else-if="!tasks?.items.length"
      title="暂无导入任务"
      description="导入文件或 URL 后，任务将显示在此处"
    />

    <div
      v-else
      class="space-y-2"
    >
      <div
        v-for="task in tasks.items"
        :key="task.id"
        class="flex items-center justify-between rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2"
      >
        <div class="flex items-center gap-3">
          <svg
            class="h-5 w-5 text-[var(--memora-muted)]"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <div>
            <p class="text-sm text-[var(--memora-text)]">
              {{ task.file_name || task.source_url || '未知文件' }}
            </p>
            <p
              v-if="task.failure_reason"
              class="text-xs text-[var(--memora-danger)]"
            >
              {{ task.failure_reason }}
            </p>
          </div>
        </div>

        <StatusBadge
          :status="task.status"
          :variant="statusVariant(task.status)"
        />
      </div>
    </div>
  </div>
</template>
