<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { searchTest } from '../api'
import type { SearchTestResponse, SearchResult } from '../types'
import BaseButton from '@/components/base/BaseButton.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import { AppError } from '@/api/errors'

const route = useRoute()
const kbId = computed(() => route.params.kbId as string)

const query = ref('')
const mode = ref<'keyword' | 'semantic' | 'hybrid'>('hybrid')
const topK = ref(8)
const loading = ref(false)
const error = ref('')
const requestId = ref('')
const results = ref<SearchTestResponse | null>(null)

async function handleSearch() {
  if (!query.value.trim()) return

  loading.value = true
  error.value = ''
  requestId.value = ''
  results.value = null

  try {
    results.value = await searchTest(kbId.value, {
      query: query.value.trim(),
      mode: mode.value,
      top_k: topK.value,
    })
  } catch (err) {
    if (err instanceof AppError) {
      error.value = err.message
      requestId.value = err.requestId || ''
    } else {
      error.value = '搜索失败，请稍后重试'
    }
  } finally {
    loading.value = false
  }
}

function getScoreDisplay(result: SearchResult): string {
  const scores: string[] = []
  if (result.keyword_score !== undefined) scores.push(`关键词: ${result.keyword_score.toFixed(3)}`)
  if (result.vector_score !== undefined) scores.push(`向量: ${result.vector_score.toFixed(3)}`)
  if (result.reranker_score !== undefined) scores.push(`Reranker: ${result.reranker_score.toFixed(3)}`)
  if (result.rrf_rank !== undefined) scores.push(`RRF: ${result.rrf_rank}`)
  return scores.join(' | ') || `排名: ${result.final_rank}`
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="border-b border-[var(--memora-border)] px-6 py-4">
      <h1 class="text-xl font-semibold text-[var(--memora-text)]">
        检索测试
      </h1>
    </div>

    <!-- Search form -->
    <div class="border-b border-[var(--memora-border)] px-6 py-4">
      <div class="flex gap-4">
        <input
          v-model="query"
          type="text"
          placeholder="输入搜索查询..."
          class="flex-1 rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          @keyup.enter="handleSearch"
        >
        <select
          v-model="mode"
          class="rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)]"
        >
          <option value="keyword">关键词</option>
          <option value="semantic">语义</option>
          <option value="hybrid">混合</option>
        </select>
        <BaseButton
          :loading="loading"
          @click="handleSearch"
        >
          搜索
        </BaseButton>
      </div>

      <!-- Error -->
      <p
        v-if="error"
        class="mt-2 text-sm text-[var(--memora-danger)]"
      >
        {{ error }}
        <span
          v-if="requestId"
          class="ml-1 text-xs text-[var(--memora-muted)]"
        >
          ({{ requestId }})
        </span>
      </p>
    </div>

    <!-- Results -->
    <div class="flex-1 overflow-y-auto p-6">
      <!-- Timing info -->
      <div
        v-if="results"
        class="mb-4 rounded-md bg-[var(--memora-bg)] px-4 py-2 text-sm text-[var(--memora-muted)]"
      >
        总耗时: {{ results.timing.total_ms }}ms
        (关键词: {{ results.timing.keyword_ms }}ms,
        向量: {{ results.timing.vector_ms }}ms,
        RRF: {{ results.timing.rrf_ms }}ms,
        Reranker: {{ results.timing.reranker_ms }}ms)
      </div>

      <!-- Final results -->
      <div v-if="results">
        <h2 class="mb-3 text-sm font-medium text-[var(--memora-text)]">
          最终结果 ({{ results.final_results.length }})
        </h2>

        <EmptyState
          v-if="results.final_results.length === 0"
          title="无搜索结果"
          description="尝试调整查询或搜索模式"
        />

        <div
          v-else
          class="space-y-3"
        >
          <div
            v-for="(result, index) in results.final_results"
            :key="result.chunk_id"
            class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
          >
            <div class="mb-2 flex items-start justify-between">
              <h3 class="text-sm font-medium text-[var(--memora-text)]">
                {{ index + 1 }}. {{ result.document_title }}
              </h3>
              <span class="text-xs text-[var(--memora-muted)]">
                {{ getScoreDisplay(result) }}
              </span>
            </div>
            <p class="text-sm text-[var(--memora-text)] line-clamp-3">
              {{ result.content }}
            </p>
            <div
              v-if="result.source_location?.section"
              class="mt-2 text-xs text-[var(--memora-muted)]"
            >
              章节: {{ result.source_location.section }}
              <span v-if="result.source_location.page"> | 页码: {{ result.source_location.page }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Empty state before search -->
      <EmptyState
        v-else-if="!loading"
        title="输入查询开始检索测试"
        description="支持关键词、语义和混合检索模式"
      />
    </div>
  </div>
</template>
