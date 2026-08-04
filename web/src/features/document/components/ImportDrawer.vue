<script setup lang="ts">
import { ref, computed } from 'vue'
import BaseDrawer from '@/components/base/BaseDrawer.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import { useImportFileMutation, useImportUrlMutation } from '../queries'

const props = defineProps<{
  open: boolean
  knowledgeBaseId: string
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
}>()

const importMode = ref<'file' | 'url'>('file')
const selectedFiles = ref<File[]>([])
const urlValue = ref('')
const duplicatePolicy = ref<'skip' | 'create_new'>('skip')
const targetDirectoryId = ref('')

const fileMutation = useImportFileMutation()
const urlMutation = useImportUrlMutation()

const maxUploadMb = computed(() => {
  return Number(import.meta.env.VITE_MAX_UPLOAD_MB || '50')
})

const allowedExtensions = ['md', 'txt', 'pdf', 'docx']

function validateFile(file: File): string | null {
  const ext = file.name.split('.').pop()?.toLowerCase()
  if (!ext || !allowedExtensions.includes(ext)) {
    return `不支持的文件类型: ${ext || '未知'}`
  }
  if (file.size > maxUploadMb.value * 1024 * 1024) {
    return `文件大小超过 ${maxUploadMb.value}MB 限制`
  }
  return null
}

function handleFileSelect(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files) return

  const validFiles: File[] = []
  for (const file of Array.from(input.files)) {
    const error = validateFile(file)
    if (error) {
      alert(error)
      continue
    }
    validFiles.push(file)
  }
  selectedFiles.value = validFiles
}

async function handleSubmit() {
  if (importMode.value === 'file') {
    if (selectedFiles.value.length === 0) return

    const formData = new FormData()
    for (const file of selectedFiles.value) {
      formData.append('files', file)
    }
    if (targetDirectoryId.value) {
      formData.append('directory_id', targetDirectoryId.value)
    }
    formData.append('duplicate_policy', duplicatePolicy.value)

    await fileMutation.mutateAsync({
      kbId: props.knowledgeBaseId,
      formData,
    })
  } else {
    if (!urlValue.value.trim()) return

    await urlMutation.mutateAsync({
      kbId: props.knowledgeBaseId,
      data: {
        url: urlValue.value.trim(),
        directory_id: targetDirectoryId.value || undefined,
        duplicate_policy: duplicatePolicy.value,
      },
    })
  }

  handleClose()
}

function handleClose() {
  emit('update:open', false)
  selectedFiles.value = []
  urlValue.value = ''
}

const isPending = computed(() => fileMutation.isPending.value || urlMutation.isPending.value)
const error = computed(() => fileMutation.error.value || urlMutation.error.value)
</script>

<template>
  <BaseDrawer
    :open="open"
    title="导入文档"
    :width="480"
    @update:open="emit('update:open', $event)"
  >
    <div class="space-y-6">
      <!-- Import mode tabs -->
      <div class="flex gap-2">
        <button
          :class="[
            'rounded-md px-4 py-2 text-sm font-medium transition-colors',
            importMode === 'file'
              ? 'bg-[var(--memora-brand-500)] text-white'
              : 'bg-[var(--memora-bg)] text-[var(--memora-muted)] hover:text-[var(--memora-text)]',
          ]"
          @click="importMode = 'file'"
        >
          文件导入
        </button>
        <button
          :class="[
            'rounded-md px-4 py-2 text-sm font-medium transition-colors',
            importMode === 'url'
              ? 'bg-[var(--memora-brand-500)] text-white'
              : 'bg-[var(--memora-bg)] text-[var(--memora-muted)] hover:text-[var(--memora-text)]',
          ]"
          @click="importMode = 'url'"
        >
          URL 导入
        </button>
      </div>

      <!-- File import -->
      <div v-if="importMode === 'file'" class="space-y-4">
        <div>
          <label class="mb-2 block text-sm font-medium text-[var(--memora-text)]">
            选择文件
          </label>
          <div class="rounded-lg border-2 border-dashed border-[var(--memora-border)] p-6 text-center">
            <input
              type="file"
              multiple
              :accept="allowedExtensions.map(e => `.${e}`).join(',')"
              class="hidden"
              @change="handleFileSelect"
            >
            <p class="mb-2 text-sm text-[var(--memora-muted)]">
              拖拽文件到此处，或点击选择
            </p>
            <p class="text-xs text-[var(--memora-muted)]">
              支持 {{ allowedExtensions.join(', ') }} 格式，最大 {{ maxUploadMb }}MB
            </p>
          </div>
          <div v-if="selectedFiles.length > 0" class="mt-3 space-y-1">
            <div
              v-for="file in selectedFiles"
              :key="file.name"
              class="flex items-center justify-between rounded-md bg-[var(--memora-bg)] px-3 py-2 text-sm"
            >
              <span class="truncate">{{ file.name }}</span>
              <span class="ml-2 text-xs text-[var(--memora-muted)]">
                {{ (file.size / 1024 / 1024).toFixed(2) }}MB
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- URL import -->
      <div v-else class="space-y-4">
        <div>
          <label
            for="import-url"
            class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
          >
            网页 URL
          </label>
          <input
            id="import-url"
            v-model="urlValue"
            type="url"
            placeholder="https://example.com/article"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
      </div>

      <!-- Duplicate policy -->
      <div>
        <label class="mb-2 block text-sm font-medium text-[var(--memora-text)]">
          重复处理策略
        </label>
        <div class="flex gap-4">
          <label class="flex items-center gap-2">
            <input
              v-model="duplicatePolicy"
              type="radio"
              value="skip"
              class="h-4 w-4 text-[var(--memora-brand-500)]"
            >
            <span class="text-sm text-[var(--memora-text)]">跳过重复</span>
          </label>
          <label class="flex items-center gap-2">
            <input
              v-model="duplicatePolicy"
              type="radio"
              value="create_new"
              class="h-4 w-4 text-[var(--memora-brand-500)]"
            >
            <span class="text-sm text-[var(--memora-text)]">创建新版本</span>
          </label>
        </div>
      </div>

      <!-- Error -->
      <p
        v-if="error"
        class="text-sm text-[var(--memora-danger)]"
      >
        {{ error.message }}
      </p>
    </div>

    <template #footer>
      <BaseButton
        variant="secondary"
        @click="handleClose"
      >
        取消
      </BaseButton>
      <BaseButton
        :disabled="importMode === 'file' ? selectedFiles.length === 0 : !urlValue.trim()"
        :loading="isPending"
        @click="handleSubmit"
      >
        导入
      </BaseButton>
    </template>
  </BaseDrawer>
</template>
