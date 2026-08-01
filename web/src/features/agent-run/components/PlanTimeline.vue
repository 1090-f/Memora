<script setup lang="ts">
import type { RuntimePlanVersion, RuntimeStep } from '@/stores/agent-runtime'

defineProps<{
  planVersions: RuntimePlanVersion[]
  steps: RuntimeStep[]
}>()
</script>

<template>
  <div
    v-if="planVersions.length > 0"
    class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-3"
  >
    <h4 class="mb-2 text-xs font-medium text-[var(--memora-muted)]">
      Plan
    </h4>

    <div
      v-for="plan in planVersions"
      :key="plan.plan_id"
      class="mb-2 last:mb-0"
    >
      <div class="flex items-center gap-2">
        <span class="text-xs font-medium text-[var(--memora-text)]">
          v{{ plan.version }}
        </span>
        <span
          v-if="plan.replan_reason"
          class="text-xs text-[var(--memora-muted)]"
        >
          ({{ plan.replan_reason }})
        </span>
      </div>
      <p class="mt-1 text-xs text-[var(--memora-text)]">
        {{ plan.goal }}
      </p>
    </div>

    <!-- Steps -->
    <div
      v-if="steps.length > 0"
      class="mt-3 space-y-1"
    >
      <div
        v-for="step in steps"
        :key="step.step_id"
        class="flex items-center gap-2 text-xs"
      >
        <span
          :class="[
            'h-2 w-2 flex-shrink-0 rounded-full',
            step.status === 'completed' && 'bg-green-500',
            step.status === 'running' && 'bg-blue-500 animate-pulse',
            step.status === 'failed' && 'bg-red-500',
            step.status === 'pending' && 'bg-gray-300',
          ]"
        />
        <span class="text-[var(--memora-text)]">
          {{ step.step_no }}. {{ step.title }}
        </span>
        <span
          v-if="step.error_message"
          class="text-[var(--memora-danger)]"
        >
          (失败)
        </span>
      </div>
    </div>
  </div>
</template>
