<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useSearchConfig, useUpdateSearchConfig } from '@/features/model-config/queries'
import BaseButton from '@/components/base/BaseButton.vue'

const props = defineProps<{
  knowledgeBaseId: string
}>()

const kbId = computed(() => props.knowledgeBaseId)
const { data: config, isLoading } = useSearchConfig(kbId)
const updateMutation = useUpdateSearchConfig(kbId)

const keywordTopK = ref(30)
const vectorTopK = ref(30)
const rrfK = ref(60)
const rrfTopK = ref(20)
const rerankerTopK = ref(8)
const rerankerThreshold = ref(0.35)
const minimumEffectiveResults = ref(1)
const rerankerModelId = ref('')

const success = ref(false)

watch(config, (c) => {
  if (c) {
    keywordTopK.value = c.keyword_top_k
    vectorTopK.value = c.vector_top_k
    rrfK.value = c.rrf_k
    rrfTopK.value = c.rrf_top_k
    rerankerTopK.value = c.reranker_top_k
    rerankerThreshold.value = c.reranker_threshold
    minimumEffectiveResults.value = c.minimum_effective_results
    rerankerModelId.value = c.reranker_model_id || ''
  }
}, { immediate: true })

async function handleSave() {
  success.value = false
  await updateMutation.mutateAsync({
    keyword_top_k: keywordTopK.value,
    vector_top_k: vectorTopK.value,
    rrf_k: rrfK.value,
    rrf_top_k: rrfTopK.value,
    reranker_top_k: rerankerTopK.value,
    reranker_threshold: rerankerThreshold.value,
    minimum_effective_results: minimumEffectiveResults.value,
    reranker_model_id: rerankerModelId.value || undefined,
  })
  success.value = true
}
</script>

<template>
  <div class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-6">
    <h3 class="mb-4 text-lg font-medium text-[var(--memora-text)]">
      搜索配置
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
      class="space-y-4"
      @submit.prevent="handleSave"
    >
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            关键词 Top-K
          </label>
          <input
            v-model.number="keywordTopK"
            type="number"
            min="1"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            向量 Top-K
          </label>
          <input
            v-model.number="vectorTopK"
            type="number"
            min="1"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            RRF K
          </label>
          <input
            v-model.number="rrfK"
            type="number"
            min="1"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            RRF Top-K
          </label>
          <input
            v-model.number="rrfTopK"
            type="number"
            min="1"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            Reranker Top-K
          </label>
          <input
            v-model.number="rerankerTopK"
            type="number"
            min="1"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            Reranker 阈值
          </label>
          <input
            v-model.number="rerankerThreshold"
            type="number"
            min="0"
            max="1"
            step="0.01"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-[var(--memora-text)]">
            最小有效结果数
          </label>
          <input
            v-model.number="minimumEffectiveResults"
            type="number"
            min="0"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>
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
