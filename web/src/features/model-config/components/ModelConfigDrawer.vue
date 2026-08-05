<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import BaseDrawer from '@/components/base/BaseDrawer.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import SecretInput from '@/components/shared/SecretInput.vue'
import { useCreateModelConfig, useUpdateModelConfig } from '../queries'
import type { ModelConfig, CreateModelConfigRequest, UpdateModelConfigRequest } from '../types'

const props = defineProps<{
  open: boolean
  editingConfig?: ModelConfig | null
  modelType: 'chat' | 'embedding' | 'reranker'
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'saved'): void
}>()

const createMutation = useCreateModelConfig()
const updateMutation = useUpdateModelConfig()

const name = ref('')
const provider = ref('openai_compatible')
const baseUrl = ref('')
const apiKey = ref('')
const timeoutSeconds = ref(60)
const retryTimes = ref(2)
const maxTokens = ref<number | null>(8192)
const temperature = ref<number | null>(0.2)
const vectorDimension = ref<number | null>(null)
const supportsToolCalling = ref(true)
const supportsStreaming = ref(true)
const isDefault = ref(false)
const enabled = ref(true)

const isEditing = computed(() => !!props.editingConfig)

watch(() => props.editingConfig, (config) => {
  if (config) {
    name.value = config.name
    provider.value = config.provider
    baseUrl.value = config.base_url
    timeoutSeconds.value = config.timeout_seconds
    retryTimes.value = config.retry_times
    maxTokens.value = config.max_tokens
    temperature.value = config.temperature
    vectorDimension.value = config.vector_dimension
    supportsToolCalling.value = config.supports_tool_calling
    supportsStreaming.value = config.supports_streaming
    isDefault.value = config.is_default
    enabled.value = config.enabled
    apiKey.value = ''
  } else {
    name.value = ''
    provider.value = 'openai_compatible'
    baseUrl.value = ''
    apiKey.value = ''
    timeoutSeconds.value = 60
    retryTimes.value = 2
    maxTokens.value = 8192
    temperature.value = 0.2
    vectorDimension.value = null
    supportsToolCalling.value = true
    supportsStreaming.value = true
    isDefault.value = false
    enabled.value = true
  }
}, { immediate: true })

async function handleSubmit() {
  if (!name.value.trim() || !baseUrl.value.trim()) return

  try {
    if (isEditing.value && props.editingConfig) {
      const data: UpdateModelConfigRequest = {
        name: name.value.trim(),
        provider: provider.value,
        base_url: baseUrl.value.trim(),
        timeout_seconds: timeoutSeconds.value,
        retry_times: retryTimes.value,
        max_tokens: maxTokens.value,
        temperature: temperature.value,
        vector_dimension: vectorDimension.value,
        supports_tool_calling: supportsToolCalling.value,
        supports_streaming: supportsStreaming.value,
        is_default: isDefault.value,
        enabled: enabled.value,
      }
      if (apiKey.value) {
        data.api_key = apiKey.value
      }
      await updateMutation.mutateAsync({ id: props.editingConfig.id, data })
    } else {
      const data: CreateModelConfigRequest = {
        model_type: props.modelType,
        name: name.value.trim(),
        provider: provider.value,
        base_url: baseUrl.value.trim(),
        api_key: apiKey.value || undefined,
        timeout_seconds: timeoutSeconds.value,
        retry_times: retryTimes.value,
        max_tokens: maxTokens.value,
        temperature: temperature.value,
        vector_dimension: vectorDimension.value,
        supports_tool_calling: supportsToolCalling.value,
        supports_streaming: supportsStreaming.value,
        is_default: isDefault.value,
        enabled: enabled.value,
      }
      await createMutation.mutateAsync(data)
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

const error = computed(() => createMutation.error.value || updateMutation.error.value)
</script>

<template>
  <BaseDrawer
    :open="open"
    :title="isEditing ? `编辑 ${modelType} 模型` : `新建 ${modelType} 模型`"
    :width="480"
    @update:open="emit('update:open', $event)"
  >
    <form
      class="space-y-4"
      @submit.prevent="handleSubmit"
    >
      <div>
        <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
          名称 <span class="text-[var(--memora-danger)]">*</span>
        </label>
        <input
          v-model="name"
          type="text"
          required
          class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
        >
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
          提供商
        </label>
        <select
          v-model="provider"
          class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)]"
        >
          <option value="openai_compatible">OpenAI 兼容</option>
        </select>
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
          API 地址 <span class="text-[var(--memora-danger)]">*</span>
        </label>
        <input
          v-model="baseUrl"
          type="url"
          required
          placeholder="https://api.example.com/v1"
          class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
        >
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
          API Key
        </label>
        <SecretInput
          v-model="apiKey"
          :configured="isEditing && !!editingConfig?.api_key_masked"
          :mode="isEditing ? 'replace' : 'create'"
          placeholder="sk-..."
        />
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            超时（秒）
          </label>
          <input
            v-model.number="timeoutSeconds"
            type="number"
            min="1"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            重试次数
          </label>
          <input
            v-model.number="retryTimes"
            type="number"
            min="0"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            最大 Token
          </label>
          <input
            v-model.number="maxTokens"
            type="number"
            min="0"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            Temperature
          </label>
          <input
            v-model.number="temperature"
            type="number"
            min="0"
            max="2"
            step="0.1"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
      </div>

      <div v-if="modelType === 'embedding'" class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            向量维度
          </label>
          <input
            v-model.number="vectorDimension"
            type="number"
            min="0"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
      </div>

      <div class="flex items-center gap-6">
        <label
          v-if="modelType === 'chat'"
          class="flex items-center gap-2"
        >
          <input
            v-model="supportsToolCalling"
            type="checkbox"
            class="h-4 w-4 rounded text-[var(--memora-brand-500)]"
          >
          <span class="text-sm text-[var(--memora-text)]">支持工具调用</span>
        </label>
        <label class="flex items-center gap-2">
          <input
            v-model="supportsStreaming"
            type="checkbox"
            class="h-4 w-4 rounded text-[var(--memora-brand-500)]"
          >
          <span class="text-sm text-[var(--memora-text)]">支持流式</span>
        </label>
        <label class="flex items-center gap-2">
          <input
            v-model="isDefault"
            type="checkbox"
            class="h-4 w-4 rounded text-[var(--memora-brand-500)]"
          >
          <span class="text-sm text-[var(--memora-text)]">默认</span>
        </label>
        <label class="flex items-center gap-2">
          <input
            v-model="enabled"
            type="checkbox"
            class="h-4 w-4 rounded text-[var(--memora-brand-500)]"
          >
          <span class="text-sm text-[var(--memora-text)]">启用</span>
        </label>
      </div>

      <p
        v-if="error"
        class="text-sm text-[var(--memora-danger)]"
      >
        {{ error.message }}
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
        {{ isEditing ? '保存' : '创建' }}
      </BaseButton>
    </template>
  </BaseDrawer>
</template>
