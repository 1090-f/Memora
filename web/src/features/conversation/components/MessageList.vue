<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import type { Message } from '../types'
import UserMessage from './UserMessage.vue'
import AssistantMessage from './AssistantMessage.vue'

const props = defineProps<{
  messages: Message[]
  loading?: boolean
}>()

const scrollContainer = ref<HTMLDivElement | null>(null)

function scrollToBottom() {
  nextTick(() => {
    if (scrollContainer.value) {
      scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight
    }
  })
}

watch(() => props.messages.length, scrollToBottom)
watch(() => props.messages[props.messages.length - 1]?.content, scrollToBottom)
</script>

<template>
  <div
    ref="scrollContainer"
    class="flex-1 overflow-y-auto px-4 py-6"
  >
    <div
      v-if="loading"
      class="flex items-center justify-center py-12"
    >
      <div class="text-sm text-[var(--memora-muted)]">
        加载中...
      </div>
    </div>

    <div
      v-else-if="messages.length === 0"
      class="flex flex-col items-center justify-center py-12"
    >
      <svg
        class="mb-4 h-12 w-12 text-[var(--memora-muted)]"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
      </svg>
      <p class="text-sm text-[var(--memora-muted)]">
        开始提问吧
      </p>
    </div>

    <div
      v-else
      class="mx-auto max-w-[900px] space-y-4"
    >
      <template v-for="message in messages" :key="message.id">
        <UserMessage
          v-if="message.role === 'user'"
          :message="message"
        />
        <AssistantMessage
          v-else
          :message="message"
        />
      </template>
    </div>
  </div>
</template>
