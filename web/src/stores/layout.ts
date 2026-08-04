import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

const DOCUMENT_SIDEBAR_WIDTH_KEY = 'memora.layout.document_sidebar_width'
const INSPECTOR_WIDTH_KEY = 'memora.layout.inspector_width'
const INSPECTOR_COLLAPSED_KEY = 'memora.layout.inspector_collapsed'

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max)
}

function loadNumber(key: string, fallback: number): number {
  const raw = localStorage.getItem(key)
  if (!raw) return fallback
  const num = Number(raw)
  return Number.isFinite(num) ? num : fallback
}

function loadBoolean(key: string, fallback: boolean): boolean {
  const raw = localStorage.getItem(key)
  if (raw === null) return fallback
  return raw === 'true'
}

export const useLayoutStore = defineStore('layout', () => {
  const document_sidebar_width = ref(
    clamp(loadNumber(DOCUMENT_SIDEBAR_WIDTH_KEY, 260), 220, 340),
  )
  const inspector_width = ref(
    clamp(loadNumber(INSPECTOR_WIDTH_KEY, 360), 300, 480),
  )
  const inspector_collapsed = ref(
    loadBoolean(INSPECTOR_COLLAPSED_KEY, false),
  )

  watch(document_sidebar_width, (val) => {
    localStorage.setItem(DOCUMENT_SIDEBAR_WIDTH_KEY, String(clamp(val, 220, 340)))
  })

  watch(inspector_width, (val) => {
    localStorage.setItem(INSPECTOR_WIDTH_KEY, String(clamp(val, 300, 480)))
  })

  watch(inspector_collapsed, (val) => {
    localStorage.setItem(INSPECTOR_COLLAPSED_KEY, String(val))
  })

  function setDocumentSidebarWidth(width: number) {
    document_sidebar_width.value = clamp(width, 220, 340)
  }

  function setInspectorWidth(width: number) {
    inspector_width.value = clamp(width, 300, 480)
  }

  function toggleInspector() {
    inspector_collapsed.value = !inspector_collapsed.value
  }

  return {
    document_sidebar_width,
    inspector_width,
    inspector_collapsed,
    setDocumentSidebarWidth,
    setInspectorWidth,
    toggleInspector,
  }
})
