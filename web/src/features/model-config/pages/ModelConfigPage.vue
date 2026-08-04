<script setup lang="ts">
import { ref, computed } from 'vue'
import { useModelConfigList, useDeleteModelConfig } from '../queries'
import ModelConfigDrawer from '../components/ModelConfigDrawer.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import LoadingSkeleton from '@/components/shared/LoadingSkeleton.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import type { ModelConfig, ModelType } from '../types'

const { data: configs, isLoading } = useModelConfigList()
const deleteMutation = useDeleteModelConfig()

const showDrawer = ref(false)
const editingConfig = ref<ModelConfig | null>(null)
const modelType = ref<ModelType>('chat')
const showDeleteConfirm = ref(false)
const deletingId = ref<string | null>(null)

const chatConfigs = computed(() => configs.value?.filter(c => c.model_type === 'chat') || [])
const embeddingConfigs = computed(() => configs.value?.filter(c => c.model_type === 'embedding') || [])
const rerankerConfigs = computed(() => configs.value?.filter(c => c.model_type === 'reranker') || [])

function handleCreate(type: ModelType) {
  editingConfig.value = null
  modelType.value = type
  showDrawer.value = true
}

function handleEdit(config: ModelConfig) {
  editingConfig.value = config
  modelType.value = config.model_type
  showDrawer.value = true
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
  } catch {
    // Error handled by mutation
  }
}

function handleSaved() {
  showDrawer.value = false
  editingConfig.value = null
}

function getTypeLabel(type: ModelType): string {
  switch (type) {
    case 'chat': return 'Chat 模型'
    case 'embedding': return 'Embedding 模型'
    case 'reranker': return 'Reranker 模型'
  }
}
</script>

<template>
  <div class="mx-auto max-w-4xl p-8">
    <h1 class="mb-8 text-2xl font-semibold text-[var(--memora-text)]">
      模型配置
    </h1>

    <LoadingSkeleton
      v-if="isLoading"
      type="list"
      :rows="6"
    />

    <template v-else>
      <!-- Model sections -->
      <div
        v-for="type in ['chat', 'embedding', 'reranker'] as ModelType[]"
        :key="type"
        class="mb-8"
      >
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-lg font-medium text-[var(--memora-text)]">
            {{ getTypeLabel(type) }}
          </h2>
          <BaseButton
            size="sm"
            @click="handleCreate(type)"
          >
            新建
          </BaseButton>
        </div>

        <EmptyState
          v-if="(type === 'chat' && !chatConfigs.length) || (type === 'embedding' && !embeddingConfigs.length) || (type === 'reranker' && !rerankerConfigs.length)"
          :title="`暂无 ${getTypeLabel(type)}`"
          description="点击新建添加模型配置"
        />

        <div
          v-else
          class="space-y-3"
        >
          <div
            v-for="config in (type === 'chat' ? chatConfigs : type === 'embedding' ? embeddingConfigs : rerankerConfigs)"
            :key="config.id"
            class="flex items-center justify-between rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
          >
            <div>
              <div class="flex items-center gap-2">
                <h3 class="text-sm font-medium text-[var(--memora-text)]">
                  {{ config.name }}
                </h3>
                <span
                  v-if="config.is_default"
                  class="rounded-full bg-[var(--memora-brand-500)]/10 px-2 py-0.5 text-xs font-medium text-[var(--memora-brand-500)]"
                >
                  默认
                </span>
                <span
                  v-if="!config.enabled"
                  class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600"
                >
                  已禁用
                </span>
              </div>
              <p class="mt-1 text-xs text-[var(--memora-muted)]">
                {{ config.base_url }}
                <span v-if="config.api_key_masked"> | {{ config.api_key_masked }}</span>
              </p>
            </div>

            <div class="flex items-center gap-2">
              <button
                class="rounded p-1 text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
                aria-label="编辑"
                @click="handleEdit(config)"
              >
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
              <button
                class="rounded p-1 text-[var(--memora-muted)] hover:bg-red-50 hover:text-[var(--memora-danger)]"
                aria-label="删除"
                @click="handleDelete(config.id)"
              >
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- Drawer -->
    <ModelConfigDrawer
      v-model:open="showDrawer"
      :editing-config="editingConfig"
      :model-type="modelType"
      @saved="handleSaved"
    />

    <!-- Delete confirmation -->
    <ConfirmDialog
      v-model:open="showDeleteConfirm"
      title="删除模型配置"
      message="确定要删除这个模型配置吗？"
      confirm-text="删除"
      :loading="deleteMutation.isPending.value"
      @confirm="confirmDelete"
    />
  </div>
</template>
