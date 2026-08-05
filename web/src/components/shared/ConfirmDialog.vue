<script setup lang="ts">
import BaseDialog from '@/components/base/BaseDialog.vue'
import BaseButton from '@/components/base/BaseButton.vue'

defineProps<{
  open: boolean
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  variant?: 'danger' | 'primary'
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'confirm'): void
  (e: 'cancel'): void
}>()

function handleCancel() {
  emit('update:open', false)
  emit('cancel')
}
</script>

<template>
  <BaseDialog
    :open="open"
    :title="title"
    @update:open="emit('update:open', $event)"
  >
    <p class="text-sm text-[var(--memora-text)]">
      {{ message }}
    </p>

    <template #footer>
      <BaseButton
        variant="secondary"
        @click="handleCancel"
      >
        {{ cancelText || '取消' }}
      </BaseButton>
      <BaseButton
        :variant="variant || 'danger'"
        :loading="loading"
        @click="emit('confirm')"
      >
        {{ confirmText || '确认' }}
      </BaseButton>
    </template>
  </BaseDialog>
</template>
