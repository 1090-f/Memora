<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

defineProps<{
  content: string
}>()

const route = useRoute()
const highlightRef = ref<HTMLDivElement | null>(null)
const quoteNotFound = ref(false)

const section = computed(() => route.query.section as string | undefined)
const quote = computed(() => route.query.quote as string | undefined)

function scrollToSection() {
  if (!highlightRef.value || !section.value) return

  const sectionElement = highlightRef.value.querySelector(`[data-section="${section.value}"]`)
  if (sectionElement) {
    sectionElement.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

function highlightQuote() {
  quoteNotFound.value = false
  if (!highlightRef.value || !quote.value) return

  // Find and highlight the quoted text using TreeWalker
  const walker = document.createTreeWalker(
    highlightRef.value,
    NodeFilter.SHOW_TEXT,
    null,
  )

  let node: Text | null
  while ((node = walker.nextNode() as Text | null)) {
    const text = node.textContent || ''
    const index = text.indexOf(quote.value)
    if (index !== -1) {
      try {
        const range = document.createRange()
        range.setStart(node, index)
        range.setEnd(node, index + quote.value.length)

        const rect = range.getBoundingClientRect()
        const containerRect = highlightRef.value.getBoundingClientRect()

        // Scroll to the highlighted text
        highlightRef.value.scrollTo({
          top: highlightRef.value.scrollTop + rect.top - containerRect.top - 100,
          behavior: 'smooth',
        })

        // Add temporary highlight
        const span = document.createElement('span')
        span.className = 'bg-yellow-200 rounded px-0.5'
        span.id = 'citation-highlight-temp'
        range.surroundContents(span)
        setTimeout(() => {
          const el = document.getElementById('citation-highlight-temp')
          if (el) {
            el.replaceWith(...Array.from(el.childNodes))
          }
        }, 5000)
        return
      } catch {
        // Range manipulation failed, fall through
      }
    }
  }

  // Quote text not found
  quoteNotFound.value = true
}

onMounted(() => {
  scrollToSection()
  highlightQuote()
})

watch([section, quote], () => {
  scrollToSection()
  highlightQuote()
})
</script>

<template>
  <div>
    <!-- Quote not found warning -->
    <div
      v-if="quoteNotFound"
      class="mb-3 rounded-md bg-yellow-50 px-3 py-2 text-sm text-yellow-800"
    >
      引用文本可能来自旧版本
    </div>

    <div
      ref="highlightRef"
      class="prose prose-sm max-w-[800px] text-[var(--memora-text)]"
      v-html="content"
    />
  </div>
</template>
