<script setup lang="ts">
import { ref, watch } from 'vue'
import BaseDrawer from '@/components/base/BaseDrawer.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import type { KnowledgeBase } from '../types'
import { useCreateKnowledgeBase, useUpdateKnowledgeBase } from '../queries'

const props = defineProps<{
  open: boolean
  editingKB?: KnowledgeBase | null
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'saved'): void
}>()

const createMutation = useCreateKnowledgeBase()
const updateMutation = useUpdateKnowledgeBase()

const name = ref('')
const description = ref('')

watch(() => props.editingKB, (kb) => {
  if (kb) {
    name.value = kb.name
    description.value = kb.description || ''
  } else {
    name.value = ''
    description.value = ''
  }
}, { immediate: true })

const isEditing = () => !!props.editingKB

async function handleSubmit() {
  if (!name.value.trim()) return

  try {
    if (isEditing() && props.editingKB) {
      await updateMutation.mutateAsync({
        id: props.editingKB.id,
        data: {
          name: name.value.trim(),
          description: description.value.trim() || undefined,
        },
      })
    } else {
      await createMutation.mutateAsync({
        name: name.value.trim(),
        description: description.value.trim() || undefined,
      })
    }
    emit('update:open', false)
    emit('saved')
  } catch {
    // Error handled by mutation
  }
}

function handleClose() {
  emit('update:open', false)
}
</script>

<template>
  <BaseDrawer
    :open="open"
    :title="isEditing() ? '编辑知识库' : '创建知识库'"
    :width="400"
    @update:open="emit('update:open', $event)"
  >
    <form
      class="space-y-4"
      @submit.prevent="handleSubmit"
    >
      <div>
        <label
          for="kb-name"
          class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
        >
          名称 <span class="text-[var(--memora-danger)]">*</span>
        </label>
        <input
          id="kb-name"
          v-model="name"
          type="text"
          required
          class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
        >
      </div>

      <div>
        <label
          for="kb-description"
          class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
        >
          描述
        </label>
        <textarea
          id="kb-description"
          v-model="description"
          rows="3"
          class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
        />
      </div>

      <p
        v-if="createMutation.error.value || updateMutation.error.value"
        class="text-sm text-[var(--memora-danger)]"
      >
        {{ (createMutation.error.value || updateMutation.error.value)?.message }}
      </p>
    </form>

    <template #footer>
      <BaseButton
        variant="secondary"
        @click="handleClose"
      >
        取消
      </BaseButton>
      <BaseButton
        :loading="createMutation.isPending.value || updateMutation.isPending.value"
        @click="handleSubmit"
      >
        {{ isEditing() ? '保存' : '创建' }}
      </BaseButton>
    </template>
  </BaseDrawer>
</template>
