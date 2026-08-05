import { computed } from 'vue'

export function useDocumentScopeChat() {
  const documentScopeEnabled = computed(() => {
    return import.meta.env.VITE_DOCUMENT_SCOPE_ENABLED === 'true'
  })

  function getDocumentScopeLabel(documentTitle?: string): string {
    if (documentScopeEnabled.value && documentTitle) {
      return `基于当前文档: ${documentTitle}`
    }
    return '基于当前知识库'
  }

  function buildQuestionPayload(query: string, documentId?: string): { query: string; document_id?: string } {
    const payload: { query: string; document_id?: string } = { query }
    if (documentScopeEnabled.value && documentId) {
      payload.document_id = documentId
    }
    return payload
  }

  return {
    documentScopeEnabled,
    getDocumentScopeLabel,
    buildQuestionPayload,
  }
}
