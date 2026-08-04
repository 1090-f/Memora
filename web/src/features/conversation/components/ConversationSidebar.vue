<script setup lang="ts">
import { computed } from 'vue'
import { useConversationList, useDeleteConversation, useCreateConversation } from '../queries'
import EmptyState from '@/components/shared/EmptyState.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import { ref } from 'vue'

const props = defineProps<{
  knowledgeBaseId: string
  selectedId?: string | null
}>()

const emit = defineEmits<{
  (e: 'select', id: string): void
  (e: 'create'): void
}>()

const kbId = computed(() => props.knowledgeBaseId)
const { data: conversations, isLoading } = useConversationList(kbId)
const createMutation = useCreateConversation(kbId)
const deleteMutation = useDeleteConversation()

const showDeleteConfirm = ref(false)
const deletingId = ref<string | null>(null)

async function handleCreate() {
  try {
    const conversation = await createMutation.mutateAsync({ title: '新会话' })
    emit('select', conversation.id)
    emit('create')
  } catch {
    // Error handled by mutation
  }
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
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-[var(--memora-border)] px-4 py-3">
      <h2 class="text-sm font-medium text-[var(--memora-text)]">
        会话
      </h2>
      <button
        class="rounded-md p-1.5 text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
        aria-label="新建会话"
        @click="handleCreate"
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
      </button>
    </div>

    <!-- Conversation list -->
    <div class="flex-1 overflow-y-auto">
      <div
        v-if="isLoading"
        class="p-4 text-sm text-[var(--memora-muted)]"
      >
        加载中...
      </div>

      <EmptyState
        v-else-if="!conversations?.items.length"
        title="暂无会话"
        description="点击 + 创建新会话"
      />

      <div
        v-else
        class="py-2"
      >
        <button
          v-for="conv in conversations.items"
          :key="conv.id"
          :class="[
            'flex w-full items-center gap-2 px-4 py-2 text-left text-sm transition-colors',
            selectedId === conv.id
              ? 'bg-[var(--memora-brand-500)]/10 text-[var(--memora-brand-500)]'
              : 'text-[var(--memora-text)] hover:bg-[var(--memora-bg)]',
          ]"
          @click="emit('select', conv.id)"
        >
          <svg class="h-4 w-4 flex-shrink-0 text-[var(--memora-muted)]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
          <span class="flex-1 truncate">{{ conv.title }}</span>
          <button
            class="flex-shrink-0 rounded p-1 text-[var(--memora-muted)] opacity-0 group-hover:opacity-100 hover:text-[var(--memora-danger)]"
            aria-label="删除"
            @click.stop="handleDelete(conv.id)"
          >
            <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </button>
      </div>
    </div>

    <!-- Delete confirmation -->
    <ConfirmDialog
      v-model:open="showDeleteConfirm"
      title="删除会话"
      message="确定要删除这个会话吗？此操作不可恢复。"
      confirm-text="删除"
      :loading="deleteMutation.isPending.value"
      @confirm="confirmDelete"
    />
  </div>
</template>
