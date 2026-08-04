<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useAgentConfig, useUpdateAgentConfig } from '@/features/model-config/queries'
import BaseButton from '@/components/base/BaseButton.vue'

const props = defineProps<{
  knowledgeBaseId: string
}>()

const kbId = computed(() => props.knowledgeBaseId)
const { data: config, isLoading } = useAgentConfig(kbId)
const updateMutation = useUpdateAgentConfig(kbId)

const name = ref('')
const systemPrompt = ref('')
const chatModelId = ref('')
const maxReactRounds = ref(8)
const maxPlanSteps = ref(5)
const maxReplans = ref(1)
const reviewerRuns = ref(1)
const maxToolCalls = ref(10)
const maxDocumentReadTokens = ref(6000)
const maxToolResultBytes = ref(1048576)
const maxRunSeconds = ref(300)
const networkEnabled = ref(false)
const memoryEnabled = ref(true)
const memoryTopK = ref(8)
const showExecutionStatus = ref(true)

const success = ref(false)

watch(config, (c) => {
  if (c) {
    name.value = c.name
    systemPrompt.value = c.system_prompt
    chatModelId.value = c.chat_model_id
    maxReactRounds.value = c.max_react_rounds
    maxPlanSteps.value = c.max_plan_steps
    maxReplans.value = c.max_replans
    reviewerRuns.value = c.reviewer_runs
    maxToolCalls.value = c.max_tool_calls
    maxDocumentReadTokens.value = c.max_document_read_tokens
    maxToolResultBytes.value = c.max_tool_result_bytes
    maxRunSeconds.value = c.max_run_seconds
    networkEnabled.value = c.network_enabled
    memoryEnabled.value = c.memory_enabled
    memoryTopK.value = c.memory_top_k
    showExecutionStatus.value = c.show_execution_status
  }
}, { immediate: true })

async function handleSave() {
  success.value = false
  await updateMutation.mutateAsync({
    name: name.value,
    system_prompt: systemPrompt.value,
    chat_model_id: chatModelId.value,
    max_react_rounds: maxReactRounds.value,
    max_plan_steps: maxPlanSteps.value,
    max_replans: maxReplans.value,
    reviewer_runs: reviewerRuns.value,
    max_tool_calls: maxToolCalls.value,
    max_document_read_tokens: maxDocumentReadTokens.value,
    max_tool_result_bytes: maxToolResultBytes.value,
    max_run_seconds: maxRunSeconds.value,
    network_enabled: networkEnabled.value,
    memory_enabled: memoryEnabled.value,
    memory_top_k: memoryTopK.value,
    show_execution_status: showExecutionStatus.value,
  })
  success.value = true
}
</script>

<template>
  <div class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-6">
    <h3 class="mb-4 text-lg font-medium text-[var(--memora-text)]">
      Agent 配置
    </h3>

    <div
      v-if="isLoading"
      class="space-y-4"
    >
      <div class="h-10 animate-pulse rounded bg-gray-200" />
      <div class="h-10 animate-pulse rounded bg-gray-200" />
    </div>

    <form
      v-else
      class="space-y-6"
      @submit.prevent="handleSave"
    >
      <!-- Model Section -->
      <div>
        <h4 class="mb-3 text-sm font-medium text-[var(--memora-text)]">
          模型
        </h4>
        <div>
          <label class="mb-1 block text-sm text-[var(--memora-muted)]">
            Agent 名称
          </label>
          <input
            v-model="name"
            type="text"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
        <div class="mt-3">
          <label class="mb-1 block text-sm text-[var(--memora-muted)]">
            系统提示词
          </label>
          <textarea
            v-model="systemPrompt"
            rows="4"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          />
        </div>
      </div>

      <!-- Budget Section -->
      <div>
        <h4 class="mb-3 text-sm font-medium text-[var(--memora-text)]">
          预算
        </h4>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="mb-1 block text-sm text-[var(--memora-muted)]">
              最大 ReAct 轮次 (≤8)
            </label>
            <input
              v-model.number="maxReactRounds"
              type="number"
              min="1"
              max="8"
              class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
            >
          </div>
          <div>
            <label class="mb-1 block text-sm text-[var(--memora-muted)]">
              最大 Plan 步骤 (≤5)
            </label>
            <input
              v-model.number="maxPlanSteps"
              type="number"
              min="1"
              max="5"
              class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
            >
          </div>
          <div>
            <label class="mb-1 block text-sm text-[var(--memora-muted)]">
              最大重规划次数 (≤1)
            </label>
            <input
              v-model.number="maxReplans"
              type="number"
              min="0"
              max="1"
              class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
            >
          </div>
          <div>
            <label class="mb-1 block text-sm text-[var(--memora-muted)]">
              最大工具调用次数 (≤10)
            </label>
            <input
              v-model.number="maxToolCalls"
              type="number"
              min="1"
              max="10"
              class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
            >
          </div>
          <div>
            <label class="mb-1 block text-sm text-[var(--memora-muted)]">
              最大运行时间（秒）
            </label>
            <input
              v-model.number="maxRunSeconds"
              type="number"
              min="1"
              class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
            >
          </div>
        </div>
      </div>

      <!-- Network & Memory Section -->
      <div>
        <h4 class="mb-3 text-sm font-medium text-[var(--memora-text)]">
          网络与记忆
        </h4>
        <div class="flex items-center gap-6">
          <label class="flex items-center gap-2">
            <input
              v-model="networkEnabled"
              type="checkbox"
              class="h-4 w-4 rounded text-[var(--memora-brand-500)]"
            >
            <span class="text-sm text-[var(--memora-text)]">启用联网</span>
          </label>
          <label class="flex items-center gap-2">
            <input
              v-model="memoryEnabled"
              type="checkbox"
              class="h-4 w-4 rounded text-[var(--memora-brand-500)]"
            >
            <span class="text-sm text-[var(--memora-text)]">启用记忆</span>
          </label>
          <label class="flex items-center gap-2">
            <input
              v-model="showExecutionStatus"
              type="checkbox"
              class="h-4 w-4 rounded text-[var(--memora-brand-500)]"
            >
            <span class="text-sm text-[var(--memora-text)]">显示执行状态</span>
          </label>
        </div>
        <div
          v-if="memoryEnabled"
          class="mt-3"
        >
          <label class="mb-1 block text-sm text-[var(--memora-muted)]">
            记忆 Top-K
          </label>
          <input
            v-model.number="memoryTopK"
            type="number"
            min="1"
            class="w-32 rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
      </div>

      <!-- MCP Tool Authorization Section (disabled until Task 12) -->
      <div>
        <h4 class="mb-3 text-sm font-medium text-[var(--memora-text)]">
          MCP 工具授权
        </h4>
        <p class="text-sm text-[var(--memora-muted)]">
          需要先配置 MCP Server（将在后续任务中启用）。
        </p>
      </div>

      <div class="flex items-center gap-4">
        <BaseButton
          :loading="updateMutation.isPending.value"
          @click="handleSave"
        >
          保存
        </BaseButton>
        <p
          v-if="success"
          class="text-sm text-[var(--memora-success)]"
        >
          保存成功
        </p>
        <p
          v-if="updateMutation.error.value"
          class="text-sm text-[var(--memora-danger)]"
        >
          {{ updateMutation.error.value.message }}
        </p>
      </div>
    </form>
  </div>
</template>
