<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useKnowledgeBaseList, useDeleteKnowledgeBase } from '../queries'
import { useWorkspaceStore } from '@/stores/workspace'
import KnowledgeBaseCard from '../components/KnowledgeBaseCard.vue'
import KnowledgeBaseFormDrawer from '../components/KnowledgeBaseFormDrawer.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import LoadingSkeleton from '@/components/shared/LoadingSkeleton.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import type { KnowledgeBase } from '../types'

const router = useRouter()
const workspaceStore = useWorkspaceStore()

const page = ref(1)
const pageSize = ref(20)
const keyword = ref('')

const query = computed(() => ({
  page: page.value,
  page_size: pageSize.value,
  keyword: keyword.value || undefined,
  sort: 'updated_at_desc',
}))

const { data, isLoading } = useKnowledgeBaseList(query)
const deleteMutation = useDeleteKnowledgeBase()

const showFormDrawer = ref(false)
const editingKB = ref<KnowledgeBase | null>(null)
const showDeleteConfirm = ref(false)
const deletingId = ref<string | null>(null)

function handleCreate() {
  editingKB.value = null
  showFormDrawer.value = true
}

function handleEdit(id: string) {
  const kb = data.value?.items.find(k => k.id === id)
  if (kb) {
    editingKB.value = kb
    showFormDrawer.value = true
  }
}

function handleSelect(id: string) {
  workspaceStore.setCurrentKbId(id)
  void router.push(`/chat/${id}`)
}

function handleDelete(id: string) {
  deletingId.value = id
  showDeleteConfirm.value = true
}

async function confirmDelete() {
  if (!deletingId.value) return
  try {
    await deleteMutation.mutateAsync(deletingId.value)
    if (workspaceStore.current_kb_id === deletingId.value) {
      workspaceStore.clearCurrentKbId()
    }
    showDeleteConfirm.value = false
    deletingId.value = null
  } catch {
    // Error handled by mutation
  }
}

function handleSaved() {
  showFormDrawer.value = false
  editingKB.value = null
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-[var(--memora-border)] px-6 py-4">
      <h1 class="text-xl font-semibold text-[var(--memora-text)]">
        知识库
      </h1>
      <BaseButton @click="handleCreate">
        创建知识库
      </BaseButton>
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto p-6">
      <LoadingSkeleton
        v-if="isLoading"
        type="card"
        :rows="6"
      />

      <EmptyState
        v-else-if="!data?.items.length"
        title="暂无知识库"
        description="创建您的第一个知识库开始使用"
      >
        <template #action>
          <BaseButton @click="handleCreate">
            创建知识库
          </BaseButton>
        </template>
      </EmptyState>

      <div
        v-else
        class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
      >
        <KnowledgeBaseCard
          v-for="kb in data.items"
          :key="kb.id"
          :knowledge-base="kb"
          @select="handleSelect"
          @edit="handleEdit"
          @delete="handleDelete"
        />
      </div>
    </div>

    <!-- Form Drawer -->
    <KnowledgeBaseFormDrawer
      v-model:open="showFormDrawer"
      :editing-k-b="editingKB"
      @saved="handleSaved"
    />

    <!-- Delete Confirmation -->
    <ConfirmDialog
      v-model:open="showDeleteConfirm"
      title="删除知识库"
      message="确定要删除这个知识库吗？此操作不可恢复，知识库中的所有文档和数据将被永久删除。"
      confirm-text="删除"
      :loading="deleteMutation.isPending.value"
      @confirm="confirmDelete"
    />
  </div>
</template>
