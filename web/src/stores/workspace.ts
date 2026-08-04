import { defineStore } from 'pinia'
import { ref } from 'vue'

const CURRENT_KB_ID_KEY = 'memora.workspace.current_kb_id'

export const useWorkspaceStore = defineStore('workspace', () => {
  const current_kb_id = ref<string | null>(
    localStorage.getItem(CURRENT_KB_ID_KEY),
  )

  function setCurrentKbId(id: string | null) {
    current_kb_id.value = id
    if (id) {
      localStorage.setItem(CURRENT_KB_ID_KEY, id)
    } else {
      localStorage.removeItem(CURRENT_KB_ID_KEY)
    }
  }

  function clearCurrentKbId() {
    current_kb_id.value = null
    localStorage.removeItem(CURRENT_KB_ID_KEY)
  }

  return {
    current_kb_id,
    setCurrentKbId,
    clearCurrentKbId,
  }
})
