<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import BaseDrawer from '@/components/base/BaseDrawer.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import SecretInput from '@/components/shared/SecretInput.vue'
import { useCreateMcpServer, useUpdateMcpServer } from '../queries'
import type { McpServer, CreateMcpServerRequest, UpdateMcpServerRequest } from '../types'

const props = defineProps<{
  open: boolean
  editingServer?: McpServer | null
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'saved'): void
}>()

const createMutation = useCreateMcpServer()
const updateMutation = useUpdateMcpServer()

const name = ref('')
const description = ref('')
const url = ref('')
const authToken = ref('')
const connectTimeout = ref(5000)
const callTimeout = ref(15000)
const maxResponseBytes = ref(1048576)
const enabled = ref(true)

const isEditing = computed(() => !!props.editingServer)

watch(() => props.editingServer, (server) => {
  if (server) {
    name.value = server.name
    description.value = server.description || ''
    url.value = server.url
    connectTimeout.value = server.connect_timeout_ms
    callTimeout.value = server.call_timeout_ms
    maxResponseBytes.value = server.max_response_bytes
    enabled.value = server.enabled
    authToken.value = ''
  } else {
    name.value = ''
    description.value = ''
    url.value = ''
    authToken.value = ''
    connectTimeout.value = 5000
    callTimeout.value = 15000
    maxResponseBytes.value = 1048576
    enabled.value = true
  }
}, { immediate: true })

async function handleSubmit() {
  if (!name.value.trim() || !url.value.trim()) return

  try {
    if (isEditing.value && props.editingServer) {
      const data: UpdateMcpServerRequest = {
        name: name.value.trim(),
        description: description.value.trim() || undefined,
        url: url.value.trim(),
        connect_timeout_ms: connectTimeout.value,
        call_timeout_ms: callTimeout.value,
        max_response_bytes: maxResponseBytes.value,
        enabled: enabled.value,
      }
      if (authToken.value) {
        data.auth = { type: 'bearer', token: authToken.value }
      }
      await updateMutation.mutateAsync({ id: props.editingServer.id, data })
    } else {
      const data: CreateMcpServerRequest = {
        name: name.value.trim(),
        description: description.value.trim() || undefined,
        transport: 'streamable_http',
        url: url.value.trim(),
        auth: authToken.value ? { type: 'bearer', token: authToken.value } : undefined,
        connect_timeout_ms: connectTimeout.value,
        call_timeout_ms: callTimeout.value,
        max_response_bytes: maxResponseBytes.value,
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
    :title="isEditing ? '编辑 MCP Server' : '新建 MCP Server'"
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
          描述
        </label>
        <input
          v-model="description"
          type="text"
          class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
        >
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
          URL <span class="text-[var(--memora-danger)]">*</span>
        </label>
        <input
          v-model="url"
          type="url"
          required
          placeholder="https://mcp.example.com/mcp"
          class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
        >
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
          Bearer Token
        </label>
        <SecretInput
          v-model="authToken"
          :configured="isEditing && !!editingServer?.auth_configured"
          :mode="isEditing ? 'replace' : 'create'"
          placeholder="输入 Token"
        />
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            连接超时（ms）
          </label>
          <input
            v-model.number="connectTimeout"
            type="number"
            min="1000"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            调用超时（ms）
          </label>
          <input
            v-model.number="callTimeout"
            type="number"
            min="1000"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
      </div>

      <label class="flex items-center gap-2">
        <input
          v-model="enabled"
          type="checkbox"
          class="h-4 w-4 rounded text-[var(--memora-brand-500)]"
        >
        <span class="text-sm text-[var(--memora-text)]">启用</span>
      </label>

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
