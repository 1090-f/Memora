<script setup lang="ts">
import { ref, computed } from 'vue'
import { useMemoryList, useUpdateMemoryStatus, useDeleteMemory } from '../queries'
import MemoryList from '../components/MemoryList.vue'
import MemoryDetailDrawer from '../components/MemoryDetailDrawer.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import LoadingSkeleton from '@/components/shared/LoadingSkeleton.vue'
import type { Memory, MemoryListQuery } from '../types'

const page = ref(1)
const pageSize = ref(20)
const statusFilter = ref('')
const typeFilter = ref('')

const query = computed<MemoryListQuery>(() => ({
  page: page.value,
  page_size: pageSize.value,
  status: statusFilter.value as MemoryListQuery['status'] || undefined,
  memory_type: typeFilter.value as MemoryListQuery['memory_type'] || undefined,
}))

const { data, isLoading } = useMemoryList(query)
const updateStatusMutation = useUpdateMemoryStatus()
const deleteMutation = useDeleteMemory()

const showDetailDrawer = ref(false)
const selectedMemory = ref<Memory | null>(null)
const showDeleteConfirm = ref(false)
const deletingId = ref<string | null>(null)

function handleSelect(id: string) {
  const memory = data.value?.items.find(m => m.id === id)
  if (memory) {
    selectedMemory.value = memory
    showDetailDrawer.value = true
  }
}

async function handleActivate(id: string) {
  await updateStatusMutation.mutateAsync({ id, status: 'active' })
}

async function handleDeactivate(id: string) {
  await updateStatusMutation.mutateAsync({ id, status: 'inactive' })
}

function handleDelete(id: string) {
  deletingId.value = id
  showDeleteConfirm.value = true
}

async function confirmDelete() {
  if (!deletingId.value) return
  try {
    await deleteMutation.mutateAsync(deletingId.value)
    showDeleteConfirm.value = false
    deletingId.value = null
    showDetailDrawer.value = false
  } catch {
    // Error handled by mutation
  }
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-[var(--memora-border)] px-6 py-4">
      <h1 class="text-xl font-semibold text-[var(--memora-text)]">
        长期记忆
      </h1>
      <div class="flex items-center gap-4">
        <select
          v-model="typeFilter"
          class="rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-1.5 text-sm outline-none focus:border-[var(--memora-brand-500)]"
        >
          <option value="">全部类型</option>
          <option value="preference">偏好</option>
          <option value="project">项目</option>
          <option value="decision">决策</option>
          <option value="goal">目标</option>
          <option value="fact">事实</option>
          <option value="progress">进展</option>
        </select>
        <select
          v-model="statusFilter"
          class="rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-1.5 text-sm outline-none focus:border-[var(--memora-brand-500)]"
        >
          <option value="">全部状态</option>
          <option value="active">活跃</option>
          <option value="inactive">停用</option>
        </select>
      </div>
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
        title="暂无记忆"
        description="与 AI 对话后，长期记忆将自动提取并显示在此处"
      />

      <MemoryList
        v-else
        :memories="data.items"
        @select="handleSelect"
        @activate="handleActivate"
        @deactivate="handleDeactivate"
        @delete="handleDelete"
      />

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

    <!-- Detail Drawer -->
    <MemoryDetailDrawer
      v-model:open="showDetailDrawer"
      :memory="selectedMemory"
      @activate="handleActivate"
      @deactivate="handleDeactivate"
      @delete="handleDelete"
    />

    <!-- Delete Confirmation -->
    <ConfirmDialog
      v-model:open="showDeleteConfirm"
      title="删除记忆"
      message="确定要删除这条记忆吗？此操作不可恢复。"
      confirm-text="删除"
      :loading="deleteMutation.isPending.value"
      @confirm="confirmDelete"
    />
  </div>
</template>
