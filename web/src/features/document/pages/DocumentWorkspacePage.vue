<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDirectoryTree, useDocumentDetail, useDeleteDocument, useProcessingState, useReindexDocument, useRetryProcessing } from '../queries'
import DocumentWorkspaceShell from '@/layouts/DocumentWorkspaceShell.vue'
import KnowledgeTree from '../components/KnowledgeTree.vue'
import DocumentToolbar from '../components/DocumentToolbar.vue'
import DocumentViewer from '../components/DocumentViewer.vue'
import DocumentProcessingState from '../components/DocumentProcessingState.vue'
import DocumentAiPanel from '../components/DocumentAiPanel.vue'
import ImportDrawer from '../components/ImportDrawer.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import BaseButton from '@/components/base/BaseButton.vue'

const route = useRoute()
const router = useRouter()

const kbId = computed(() => route.params.kbId as string)
const documentId = computed(() => route.params.documentId as string | undefined)

// Directory tree
const { data: directories, isLoading: dirsLoading } = useDirectoryTree(kbId)

// Document detail
const { data: document } = useDocumentDetail(documentId)

// Processing state
const { data: processingState } = useProcessingState(documentId)

// Mutations
const deleteMutation = useDeleteDocument()
const reindexMutation = useReindexDocument()
const retryMutation = useRetryProcessing()

// UI state
const showDeleteConfirm = ref(false)
const showImportDrawer = ref(false)

function handleSelectDocument(id: string) {
  void router.push(`/kb/${kbId.value}/docs/${id}`)
}

async function handleDelete() {
  if (!documentId.value) return
  try {
    await deleteMutation.mutateAsync(documentId.value)
    showDeleteConfirm.value = false
    void router.push(`/kb/${kbId.value}/docs`)
  } catch {
    // Error handled by mutation
  }
}

async function handleReindex() {
  if (!documentId.value) return
  await reindexMutation.mutateAsync({ id: documentId.value })
}

async function handleRetry() {
  if (!documentId.value) return
  await retryMutation.mutateAsync(documentId.value)
}
</script>

<template>
  <DocumentWorkspaceShell>
    <template #sidebar>
      <div class="flex h-full flex-col">
        <div class="flex items-center justify-between border-b border-[var(--memora-border)] px-4 py-3">
          <h2 class="text-sm font-medium text-[var(--memora-text)]">
            目录
          </h2>
          <BaseButton
            size="sm"
            @click="showImportDrawer = true"
          >
            导入
          </BaseButton>
        </div>
        <div class="flex-1 overflow-y-auto">
          <KnowledgeTree
            v-if="directories"
            :directories="directories"
            :selected-id="document?.directory_id"
            @select-document="handleSelectDocument"
          />
          <div
            v-else-if="dirsLoading"
            class="p-4 text-sm text-[var(--memora-muted)]"
          >
            加载中...
          </div>
        </div>
      </div>
    </template>

    <template #toolbar>
      <DocumentToolbar
        :document="document || null"
        @delete="showDeleteConfirm = true"
        @reindex="handleReindex"
      />
    </template>

    <template #viewer>
      <DocumentViewer
        :document="document || null"
        :processing-status="processingState?.processing_status || document?.processing_status"
      />
      <DocumentProcessingState
        v-if="processingState && processingState.processing_status !== 'succeeded'"
        :state="processingState"
        @retry="handleRetry"
      />
    </template>

    <template #inspector>
      <DocumentAiPanel
        :document-title="document?.title"
        :knowledge-base-id="kbId"
      />
    </template>
  </DocumentWorkspaceShell>

  <!-- Import Drawer -->
  <ImportDrawer
    v-model:open="showImportDrawer"
    :knowledge-base-id="kbId"
  />

  <!-- Delete confirmation -->
  <ConfirmDialog
    v-model:open="showDeleteConfirm"
    title="删除文档"
    message="确定要删除这个文档吗？此操作不可恢复。"
    confirm-text="删除"
    :loading="deleteMutation.isPending.value"
    @confirm="handleDelete"
  />
</template>
