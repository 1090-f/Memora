<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAgentRunDetail, useRouterDecision, usePlans, useRounds, useToolCalls, useRunCitations } from '../queries'
import StatusBadge from '@/components/shared/StatusBadge.vue'
import LoadingSkeleton from '@/components/shared/LoadingSkeleton.vue'
import CitationList from '../components/CitationList.vue'

const route = useRoute()
const router = useRouter()

const runId = computed(() => route.params.runId as string)

const { data: run, isLoading } = useAgentRunDetail(runId)
const { data: routerDecision } = useRouterDecision(runId)
const { data: plans } = usePlans(runId)
const { data: rounds } = useRounds(runId)
const { data: toolCalls } = useToolCalls(runId)
const { data: citations } = useRunCitations(runId)

const statusVariant = (status: string | null) => {
  switch (status) {
    case 'completed': return 'success'
    case 'failed': return 'danger'
    case 'running': return 'info'
    case 'cancelled': return 'warning'
    default: return 'default'
  }
}

function formatDuration(ms: number | null): string {
  if (!ms) return '-'
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="border-b border-[var(--memora-border)] px-6 py-4">
      <div class="flex items-center gap-3">
        <button
          class="rounded p-1 text-[var(--memora-muted)] hover:bg-[var(--memora-bg)] hover:text-[var(--memora-text)]"
          aria-label="返回"
          @click="router.back()"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <h1 class="text-xl font-semibold text-[var(--memora-text)]">
          运行详情
        </h1>
      </div>
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto p-6">
      <LoadingSkeleton
        v-if="isLoading"
        type="list"
        :rows="5"
      />

      <template v-else-if="run">
        <!-- Summary -->
        <div class="mb-6 rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4">
          <div class="mb-3 flex items-center gap-3">
            <StatusBadge
              :status="run.status"
              :variant="statusVariant(run.status)"
            />
            <span
              v-if="run.execution_mode"
              class="text-sm text-[var(--memora-muted)]"
            >
              {{ run.execution_mode === 'react' ? 'ReAct' : 'Plan-Execute' }}
            </span>
            <span class="text-sm text-[var(--memora-muted)]">
              {{ formatDuration(run.duration_ms) }}
            </span>
          </div>

          <p class="mb-3 text-sm text-[var(--memora-text)]">
            {{ run.query }}
          </p>

          <div class="grid grid-cols-3 gap-4 text-sm">
            <div>
              <span class="text-[var(--memora-muted)]">输入 Token:</span>
              <span class="ml-1 text-[var(--memora-text)]">{{ run.input_tokens ?? '-' }}</span>
            </div>
            <div>
              <span class="text-[var(--memora-muted)]">输出 Token:</span>
              <span class="ml-1 text-[var(--memora-text)]">{{ run.output_tokens ?? '-' }}</span>
            </div>
            <div>
              <span class="text-[var(--memora-muted)]">总计 Token:</span>
              <span class="ml-1 text-[var(--memora-text)]">{{ run.total_tokens ?? '-' }}</span>
            </div>
          </div>

          <div
            v-if="run.error_message"
            class="mt-3 rounded-md bg-red-50 p-3 text-sm text-[var(--memora-danger)]"
          >
            {{ run.error_message }}
          </div>
        </div>

        <!-- Router Decision -->
        <div
          v-if="routerDecision"
          class="mb-6 rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
        >
          <h3 class="mb-2 text-sm font-medium text-[var(--memora-text)]">
            Router 决策
          </h3>
          <div class="flex items-center gap-2">
            <span
              :class="[
                'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                routerDecision.execution_mode === 'react' ? 'bg-blue-100 text-blue-800' : 'bg-purple-100 text-purple-800',
              ]"
            >
              {{ routerDecision.execution_mode === 'react' ? 'ReAct' : 'Plan-Execute' }}
            </span>
            <span class="text-xs text-[var(--memora-muted)]">
              置信度: {{ (routerDecision.confidence * 100).toFixed(0) }}%
            </span>
          </div>
          <p class="mt-2 text-sm text-[var(--memora-text)]">
            {{ routerDecision.reason_summary }}
          </p>
        </div>

        <!-- Plans -->
        <div
          v-if="plans && plans.length > 0"
          class="mb-6 rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
        >
          <h3 class="mb-2 text-sm font-medium text-[var(--memora-text)]">
            Plan 版本
          </h3>
          <div
            v-for="plan in plans"
            :key="plan.id"
            class="mb-2 rounded-md bg-[var(--memora-bg)] p-3 last:mb-0"
          >
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-[var(--memora-text)]">
                v{{ plan.version }}
              </span>
              <StatusBadge :status="plan.status" />
              <span
                v-if="plan.is_current"
                class="text-xs text-[var(--memora-brand-500)]"
              >
                (当前)
              </span>
            </div>
            <p class="mt-1 text-sm text-[var(--memora-text)]">
              {{ plan.goal }}
            </p>
            <p
              v-if="plan.replan_reason"
              class="mt-1 text-xs text-[var(--memora-muted)]"
            >
              重规划原因: {{ plan.replan_reason }}
            </p>
          </div>
        </div>

        <!-- Rounds -->
        <div
          v-if="rounds && rounds.length > 0"
          class="mb-6 rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
        >
          <h3 class="mb-2 text-sm font-medium text-[var(--memora-text)]">
            ReAct 轮次
          </h3>
          <div class="space-y-2">
            <div
              v-for="round in rounds"
              :key="round.round_no"
              class="flex items-center gap-3 rounded-md bg-[var(--memora-bg)] p-3"
            >
              <span class="text-sm font-medium text-[var(--memora-muted)]">
                #{{ round.round_no }}
              </span>
              <StatusBadge :status="round.status" />
              <span class="flex-1 text-sm text-[var(--memora-text)]">
                {{ round.action_summary || '-' }}
              </span>
              <span
                v-if="round.tool_name"
                class="text-xs text-[var(--memora-muted)]"
              >
                {{ round.tool_name }}
              </span>
            </div>
          </div>
        </div>

        <!-- Tool Calls -->
        <div
          v-if="toolCalls && toolCalls.length > 0"
          class="mb-6 rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
        >
          <h3 class="mb-2 text-sm font-medium text-[var(--memora-text)]">
            工具调用
          </h3>
          <div class="space-y-2">
            <div
              v-for="tool in toolCalls"
              :key="tool.tool_call_id"
              class="rounded-md bg-[var(--memora-bg)] p-3"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-[var(--memora-text)]">
                    {{ tool.tool_name }}
                  </span>
                  <span class="text-xs text-[var(--memora-muted)]">
                    ({{ tool.tool_type }})
                  </span>
                  <StatusBadge :status="tool.status" />
                </div>
                <span
                  v-if="tool.duration_ms"
                  class="text-xs text-[var(--memora-muted)]"
                >
                  {{ tool.duration_ms }}ms
                </span>
              </div>
              <p
                v-if="tool.input_summary"
                class="mt-1 text-xs text-[var(--memora-muted)]"
              >
                输入: {{ tool.input_summary }}
              </p>
              <p
                v-if="tool.output_summary"
                class="mt-1 text-xs text-[var(--memora-muted)]"
              >
                输出: {{ tool.output_summary }}
              </p>
            </div>
          </div>
        </div>

        <!-- Citations -->
        <div
          v-if="citations && citations.length > 0"
          class="mb-6 rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
        >
          <h3 class="mb-2 text-sm font-medium text-[var(--memora-text)]">
            引用
          </h3>
          <CitationList :citations="citations" />
        </div>

        <!-- Final Result -->
        <div
          v-if="run.final_result"
          class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-4"
        >
          <h3 class="mb-2 text-sm font-medium text-[var(--memora-text)]">
            最终结果
          </h3>
          <p class="text-sm text-[var(--memora-text)] whitespace-pre-wrap">
            {{ run.final_result }}
          </p>
        </div>
      </template>
    </div>
  </div>
</template>
