/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string
  readonly VITE_MAX_UPLOAD_MB: string
  readonly VITE_SSE_RESUME_ENABLED: string
  readonly VITE_DOCUMENT_SCOPE_ENABLED: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
