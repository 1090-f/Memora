<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import SearchConfigForm from '../components/SearchConfigForm.vue'
import AgentConfigForm from '../components/AgentConfigForm.vue'

const route = useRoute()
const kbId = computed(() => route.params.kbId as string)
</script>

<template>
  <div class="mx-auto max-w-4xl p-8">
    <h1 class="mb-8 text-2xl font-semibold text-[var(--memora-text)]">
      知识库设置
    </h1>

    <div class="space-y-8">
      <SearchConfigForm :knowledge-base-id="kbId" />
      <AgentConfigForm :knowledge-base-id="kbId" />
    </div>
  </div>
</template>
