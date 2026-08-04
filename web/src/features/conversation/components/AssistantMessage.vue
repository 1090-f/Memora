<script setup lang="ts">
import { computed } from 'vue'
import { renderMarkdown, processExternalLinks } from '@/utils/markdown'
import CitationList from '@/features/agent-run/components/CitationList.vue'
import type { Message } from '../types'

const props = defineProps<{
  message: Message
}>()

const renderedContent = computed(() => {
  if (!props.message.content) return ''
  return processExternalLinks(renderMarkdown(props.message.content))
})

const isStreaming = computed(() => props.message.status === 'streaming')
const isFailed = computed(() => props.message.status === 'failed')
const hasCitations = computed(() => props.message.citations && props.message.citations.length > 0)
</script>

<template>
  <div class="flex justify-start">
    <div class="max-w-[70%]">
      <!-- Streaming indicator -->
      <div
        v-if="isStreaming && !message.content"
        class="flex items-center gap-2 rounded-lg bg-[var(--memora-surface)] px-4 py-2 text-sm text-[var(--memora-muted)]"
      >
        <div class="h-2 w-2 animate-pulse rounded-full bg-[var(--memora-brand-500)]" />
        正在思考...
      </div>

      <!-- Content -->
      <div
        v-else-if="message.content"
        class="rounded-lg bg-[var(--memora-surface)] px-4 py-2 text-sm text-[var(--memora-text)]"
      >
        <!-- eslint-disable-next-line vue/no-v-html -->
        <div class="prose prose-sm max-w-none" v-html="renderedContent" />

        <!-- Citations -->
        <CitationList
          v-if="hasCitations"
          :citations="message.citations || []"
        />
      </div>

      <!-- Failed state -->
      <div
        v-if="isFailed"
        class="mt-1 text-xs text-[var(--memora-danger)]"
      >
        回答失败
      </div>
    </div>
  </div>
</template>
