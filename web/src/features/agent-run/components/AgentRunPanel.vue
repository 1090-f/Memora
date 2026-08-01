<script setup lang="ts">
import { useAgentRuntimeStore } from '@/stores/agent-runtime'
import RouterSummary from './RouterSummary.vue'
import PlanTimeline from './PlanTimeline.vue'
import ReactRounds from './ReactRounds.vue'
import ToolCallList from './ToolCallList.vue'
import UsageSummary from './UsageSummary.vue'
import EmptyState from '@/components/shared/EmptyState.vue'

const runtimeStore = useAgentRuntimeStore()
</script>

<template>
  <div class="flex h-full flex-col overflow-y-auto p-3">
    <EmptyState
      v-if="runtimeStore.status === 'idle'"
      title="提交问题后显示执行过程"
      description="Agent 运行状态和工具调用将在此处展示"
    />

    <div
      v-else
      class="space-y-3"
    >
      <!-- Status indicator -->
      <div
        v-if="runtimeStore.status === 'streaming'"
        class="flex items-center gap-2 rounded-lg bg-blue-50 px-3 py-2 text-xs text-blue-800"
      >
        <div class="h-2 w-2 animate-pulse rounded-full bg-blue-500" />
        执行中...
      </div>

      <div
        v-if="runtimeStore.status === 'completed'"
        class="rounded-lg bg-green-50 px-3 py-2 text-xs text-green-800"
      >
        执行完成
      </div>

      <div
        v-if="runtimeStore.status === 'failed'"
        class="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-800"
      >
        执行失败: {{ runtimeStore.error?.message || '未知错误' }}
      </div>

      <div
        v-if="runtimeStore.status === 'cancelled'"
        class="rounded-lg bg-yellow-50 px-3 py-2 text-xs text-yellow-800"
      >
        已取消
      </div>

      <!-- Router -->
      <RouterSummary
        :mode="runtimeStore.execution_mode"
        :reason="runtimeStore.router_reason_summary"
      />

      <!-- Plan (if plan_execute mode) -->
      <PlanTimeline
        v-if="runtimeStore.execution_mode === 'plan_execute'"
        :plan-versions="runtimeStore.plan_versions"
        :steps="runtimeStore.steps"
      />

      <!-- ReAct Rounds (if react mode) -->
      <ReactRounds
        v-if="runtimeStore.execution_mode === 'react'"
        :rounds="runtimeStore.rounds"
      />

      <!-- Tool Calls -->
      <ToolCallList :tool-calls="runtimeStore.tool_calls" />

      <!-- Usage -->
      <UsageSummary
        :usage="runtimeStore.usage"
        :memory-count="runtimeStore.memory_count"
      />
    </div>
  </div>
</template>
