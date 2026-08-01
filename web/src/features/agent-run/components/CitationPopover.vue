<script setup lang="ts">
import { ref } from 'vue'
import type { Citation, KnowledgeBaseCitation, NetworkCitation } from '@/features/conversation/types'

defineProps<{
  citation: Citation
}>()

const isOpen = ref(false)

function isKnowledgeBase(citation: Citation): citation is KnowledgeBaseCitation {
  return citation.source_type === 'knowledge_base'
}

function isNetwork(citation: Citation): citation is NetworkCitation {
  return citation.source_type === 'network'
}

function openDocument(citation: KnowledgeBaseCitation) {
  const section = citation.source_location?.section
    ? `&section=${encodeURIComponent(citation.source_location.section)}`
    : ''
  const quote = citation.quoted_text
    ? `&quote=${encodeURIComponent(citation.quoted_text)}`
    : ''
  const url = `/kb/${citation.knowledge_base_id}/docs/${citation.document_id}?fromConversation=true${section}${quote}`
  void window.open(url, '_blank')
}
</script>

<template>
  <div
    class="relative inline-block"
    @mouseenter="isOpen = true"
    @mouseleave="isOpen = false"
  >
    <button
      :class="[
        'inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs font-medium transition-colors',
        isKnowledgeBase(citation)
          ? 'bg-[var(--memora-brand-500)]/10 text-[var(--memora-brand-500)] hover:bg-[var(--memora-brand-500)]/20'
          : 'bg-green-100 text-green-700 hover:bg-green-200',
      ]"
    >
      <svg
        v-if="isKnowledgeBase(citation)"
        class="h-3 w-3"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
      </svg>
      <svg
        v-else
        class="h-3 w-3"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
      </svg>
      {{ isKnowledgeBase(citation) ? citation.document_title : citation.site_name }}
    </button>

    <!-- Popover -->
    <div
      v-if="isOpen"
      class="absolute bottom-full left-0 z-50 mb-2 w-72 rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-3 shadow-lg"
    >
      <div v-if="isKnowledgeBase(citation)">
        <p class="mb-1 text-sm font-medium text-[var(--memora-text)]">
          {{ citation.document_title }}
        </p>
        <p class="mb-2 text-xs text-[var(--memora-muted)] line-clamp-3">
          {{ citation.quoted_text }}
        </p>
        <div
          v-if="citation.source_location?.section"
          class="mb-2 text-xs text-[var(--memora-muted)]"
        >
          章节: {{ citation.source_location.section }}
          <span v-if="citation.source_location.page"> | 页码: {{ citation.source_location.page }}</span>
        </div>
        <p class="mb-2 text-xs text-[var(--memora-muted)]">
          文档更新: {{ new Date(citation.document_updated_at).toLocaleDateString() }}
        </p>
        <button
          class="w-full rounded-md bg-[var(--memora-brand-500)]/10 px-3 py-1.5 text-xs font-medium text-[var(--memora-brand-500)] hover:bg-[var(--memora-brand-500)]/20"
          @click="openDocument(citation)"
        >
          打开文档
        </button>
      </div>

      <div v-else-if="isNetwork(citation)">
        <p class="mb-1 text-sm font-medium text-[var(--memora-text)]">
          {{ citation.title }}
        </p>
        <p class="mb-2 text-xs text-[var(--memora-muted)]">
          {{ citation.site_name }}
        </p>
        <a
          :href="citation.url"
          target="_blank"
          rel="noopener noreferrer"
          class="text-xs text-[var(--memora-brand-500)] hover:underline"
        >
          {{ citation.url }}
        </a>
      </div>
    </div>
  </div>
</template>
