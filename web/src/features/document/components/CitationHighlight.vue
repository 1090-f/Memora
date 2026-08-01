<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

defineProps<{
  content: string
}>()

const route = useRoute()
const highlightRef = ref<HTMLDivElement | null>(null)

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
  if (!highlightRef.value || !quote.value) return

  // Find and highlight the quoted text using text content search
  const textContent = highlightRef.value.textContent || ''
  const index = textContent.indexOf(quote.value)
  if (index !== -1) {
    // Simple scroll to approximate position
    const approximatePosition = (index / textContent.length) * highlightRef.value.scrollHeight
    highlightRef.value.scrollTo({
      top: approximatePosition - 100,
      behavior: 'smooth',
    })
  }
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
  <div
    ref="highlightRef"
    class="prose prose-sm max-w-[800px] text-[var(--memora-text)]"
    v-html="content"
  />
</template>
