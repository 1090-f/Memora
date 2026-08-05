<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAgentRunList } from '../queries'
import RunFilters from '../components/RunFilters.vue'
import RunSummaryCard from '../components/RunSummaryCard.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import LoadingSkeleton from '@/components/shared/LoadingSkeleton.vue'
import type { AgentRunListQuery } from '../types'

const router = useRouter()

const page = ref(1)
const pageSize = ref(20)
const status = ref('')
const mode = ref('')

const query = computed<AgentRunListQuery>(() => ({
  page: page.value,
  page_size: pageSize.value,
  status: status.value || undefined,
  execution_mode: mode.value || undefined,
  sort: 'created_at_desc',
}))

const { data, isLoading } = useAgentRunList(query)

function handleFilterChange(filters: { status?: string; execution_mode?: string }) {
  status.value = filters.status || ''
  mode.value = filters.execution_mode || ''
  page.value = 1
}

function handleSelect(id: string) {
  void router.push(`/runs/${id}`)
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-[var(--memora-border)] px-6 py-4">
      <h1 class="text-xl font-semibold text-[var(--memora-text)]">
        Agent 运行记录
      </h1>
      <RunFilters
        :initial-status="status"
        :initial-mode="mode"
        @change="handleFilterChange"
      />
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto p-6">
      <LoadingSkeleton
        v-if="isLoading"
        type="list"
        :rows="5"
      />

      <EmptyState
        v-else-if="!data?.items.length"
        title="暂无运行记录"
        description="在智能问答中提交问题后，运行记录将显示在此处"
      />

      <div
        v-else
        class="space-y-3"
      >
        <RunSummaryCard
          v-for="run in data.items"
          :key="run.id"
          :run="run"
          @select="handleSelect"
        />
      </div>

      <!-- Pagination -->
      <div
        v-if="data && data.total > pageSize"
        class="mt-6 flex items-center justify-center gap-2"
      >
        <button
          :disabled="page <= 1"
          class="rounded-md px-3 py-1.5 text-sm text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] disabled:opacity-50"
          @click="page--"
        >
          上一页
        </button>
        <span class="text-sm text-[var(--memora-muted)]">
          {{ page }} / {{ Math.ceil(data.total / pageSize) }}
        </span>
        <button
          :disabled="page >= Math.ceil(data.total / pageSize)"
          class="rounded-md px-3 py-1.5 text-sm text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] disabled:opacity-50"
          @click="page++"
        >
          下一页
        </button>
      </div>
    </div>
  </div>
</template>
