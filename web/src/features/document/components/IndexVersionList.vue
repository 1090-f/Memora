<script setup lang="ts">
import { computed } from 'vue'
import { useIndexVersions } from '../queries'
import StatusBadge from '@/components/shared/StatusBadge.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import BaseButton from '@/components/base/BaseButton.vue'

const props = defineProps<{
  documentId: string
}>()

const docId = computed(() => props.documentId)
const { data: versions, isLoading } = useIndexVersions(docId)

const statusVariant = (status: string) => {
  switch (status) {
    case 'active': return 'success'
    case 'failed': return 'danger'
    case 'processing': return 'info'
    default: return 'default'
  }
}

defineEmits<{
  (e: 'reindex'): void
}>()
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <h3 class="text-sm font-medium text-[var(--memora-text)]">
        索引版本
      </h3>
      <BaseButton
        size="sm"
        variant="secondary"
        @click="$emit('reindex')"
      >
        重新索引
      </BaseButton>
    </div>

    <LoadingSkeleton
      v-if="isLoading"
      type="list"
      :rows="2"
    />

    <EmptyState
      v-else-if="!versions?.length"
      title="暂无索引版本"
    />

    <div
      v-else
      class="space-y-2"
    >
      <div
        v-for="version in versions"
        :key="version.id"
        class="flex items-center justify-between rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2"
      >
        <div>
          <p class="text-sm text-[var(--memora-text)]">
            版本 {{ version.version }}
          </p>
          <p class="text-xs text-[var(--memora-muted)]">
            {{ new Date(version.created_at).toLocaleString() }}
          </p>
        </div>
        <StatusBadge
          :status="version.status"
          :variant="statusVariant(version.status)"
        />
      </div>
    </div>
  </div>
</template>
