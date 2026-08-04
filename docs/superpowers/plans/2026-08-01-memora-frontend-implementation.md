# Memora P0 Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the complete desktop-only Memora P0 frontend as a Vue 3 SPA, including authentication, knowledge and document workspaces, streaming Agent chat, run observability, Memory, model configuration, and read-only MCP management.

**Architecture:** Create an independent `web/` application that consumes the existing `/api/v1` REST envelope and AgentRun SSE stream. Keep server state in TanStack Vue Query, client/session state in Pinia, and live events in a dedicated runtime store; organize code by feature so each module has focused API, query, composable, component, and page boundaries.

**Tech Stack:** Vue 3, Vite, TypeScript, Vue Router, Pinia, TanStack Vue Query, Tailwind CSS, Reka UI, DOMPurify, markdown-it, Zod, Lucide Vue Next, native Fetch API, pnpm.

## Global Constraints

- Read `docs/superpowers/specs/2026-08-01-memora-frontend-design.md` and `AI智能知识库_API接口文档_P0_轻量化版.md` before editing.
- Build only the desktop Web application; support viewport widths of 1280px, 1440px, and 1920px. Below 1280px, show an unsupported-width notice.
- Documents are read-only. Do not add a rich-text editor, document rewriting, or collaborative editing.
- Users never choose `react` or `plan_execute`; the Router Agent decides and the frontend only renders its result.
- Never render or persist model hidden chain-of-thought. Render only structured summaries returned by the API.
- Keep API DTO fields in `snake_case`; do not install a global JSON case converter.
- The frontend must not send `user_id`, runtime `agent_run_id`, `allowed_tool_names`, or any identity/authorization field owned by the backend.
- Use `sessionStorage` for the P0 Access Token. Never use `localStorage` for tokens and never persist passwords or secrets.
- Sanitize all Markdown/HTML output with DOMPurify before rendering. Do not render unsanitized `v-html`.
- Preserve the repository's current rule that test files are not retained. Each task uses type checking, linting, production build, and explicit manual acceptance instead of committed Vitest or Playwright files.
- Do not describe static checks as runtime acceptance. Report typecheck, lint, build, and manual API verification separately.
- Use `pnpm` and commit the generated lockfile.
- Work in a dedicated feature branch or worktree. Preserve unrelated dirty changes in the parent checkout.
- Commit only files belonging to the current task. Use the exact commit message shown at the end of each task.

---

## File and Module Map

Create the following top-level structure. A task may add focused files inside a listed feature, but must not merge unrelated responsibilities into one large file.

```text
web/
├── public/
├── src/
│   ├── api/
│   │   ├── client.ts              # REST request, envelope unwrap, auth, errors
│   │   ├── errors.ts              # AppError and error-code helpers
│   │   ├── pagination.ts          # PageQuery and PageData
│   │   └── sse.ts                 # authenticated SSE Fetch parser
│   ├── app/
│   │   ├── App.vue                # application root
│   │   └── providers.ts           # Pinia and Vue Query installation
│   ├── components/
│   │   ├── base/                  # wrapped Reka primitives
│   │   └── shared/                # empty, error, loading, status, secret input
│   ├── features/
│   │   ├── auth/
│   │   ├── knowledge-base/
│   │   ├── document/
│   │   ├── search/
│   │   ├── conversation/
│   │   ├── agent-run/
│   │   ├── memory/
│   │   ├── mcp/
│   │   ├── model-config/
│   │   └── user/
│   ├── layouts/
│   │   ├── AppShell.vue
│   │   ├── ChatWorkspaceShell.vue
│   │   ├── DocumentWorkspaceShell.vue
│   │   └── UnsupportedViewport.vue
│   ├── router/index.ts
│   ├── stores/
│   │   ├── auth.ts
│   │   ├── workspace.ts
│   │   ├── layout.ts
│   │   └── agent-runtime.ts
│   ├── styles/
│   │   ├── index.css
│   │   └── tokens.css
│   ├── types/
│   │   ├── common.ts
│   │   └── env.d.ts
│   ├── utils/
│   │   ├── markdown.ts
│   │   ├── format.ts
│   │   └── redact.ts
│   └── main.ts
├── .env.example
├── eslint.config.js
├── index.html
├── package.json
├── pnpm-lock.yaml
├── tsconfig.app.json
├── tsconfig.json
├── tsconfig.node.json
└── vite.config.ts
```

## Shared Interfaces

Define these once and use the exact names throughout the plan.

```ts
export interface ApiEnvelope<T> {
  code: string
  message: string
  data: T
  request_id: string
}

export interface PageData<T> {
  items: T[]
  page: number
  page_size: number
  total: number
}

export interface PageQuery {
  page?: number
  page_size?: number
  keyword?: string
  sort?: string
}

export class AppError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly httpStatus: number,
    public readonly requestId?: string,
    public readonly details?: unknown,
  ) {
    super(message)
    this.name = 'AppError'
  }
}
```

---

### Task 1: Scaffold the Vue Application and Quality Gates

**Files:**
- Create: `web/package.json`
- Create: `web/pnpm-lock.yaml`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/tsconfig.app.json`
- Create: `web/tsconfig.node.json`
- Create: `web/eslint.config.js`
- Create: `web/.env.example`
- Create: `web/index.html`
- Create: `web/src/main.ts`
- Create: `web/src/app/App.vue`
- Create: `web/src/app/providers.ts`
- Create: `web/src/router/index.ts`
- Create: `web/src/features/auth/pages/LoginPage.vue`
- Create: `web/src/styles/index.css`
- Create: `web/src/styles/tokens.css`
- Create: `web/src/types/env.d.ts`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: none.
- Produces: a buildable Vue SPA, `installProviders(app: App): void`, router instance, global CSS tokens, and scripts used by every later task.

- [ ] **Step 1: Create the Vite Vue TypeScript project**

Run from the repository root:

```bash
pnpm create vite@latest web --template vue-ts
cd web
pnpm install
```

Expected: `web/` contains a Vue TypeScript starter and `pnpm-lock.yaml`.

- [ ] **Step 2: Install the approved runtime dependencies**

Run:

```bash
pnpm add vue-router pinia @tanstack/vue-query @tanstack/vue-virtual reka-ui tailwindcss @tailwindcss/vite dompurify markdown-it zod lucide-vue-next
pnpm add -D eslint @eslint/js eslint-plugin-vue typescript-eslint vue-tsc @types/markdown-it
```

Expected: all packages resolve and are recorded in `package.json` and the lockfile.

- [ ] **Step 3: Define exact package scripts**

Ensure `web/package.json` contains these scripts:

```json
{
  "scripts": {
    "dev": "vite",
    "typecheck": "vue-tsc --noEmit",
    "lint": "eslint . --max-warnings=0",
    "build": "pnpm typecheck && vite build",
    "preview": "vite preview"
  }
}
```

- [ ] **Step 4: Configure Vite, Tailwind, and the development API proxy**

Create `web/vite.config.ts`:

```ts
import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/health': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
```

- [ ] **Step 5: Create providers and the empty route shell**

Create `web/src/app/providers.ts`:

```ts
import type { App } from 'vue'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createPinia } from 'pinia'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 30_000 },
    mutations: { retry: 0 },
  },
})

export function installProviders(app: App): void {
  app.use(createPinia())
  app.use(VueQueryPlugin, { queryClient })
}
```

Create `web/src/main.ts`:

```ts
import { createApp } from 'vue'
import App from '@/app/App.vue'
import { installProviders } from '@/app/providers'
import router from '@/router'
import '@/styles/index.css'

const app = createApp(App)
installProviders(app)
app.use(router)
app.mount('#app')
```

Create a router with `/` redirecting to `/login` and `/login` lazy-loading `LoginPage.vue`. In this task, `LoginPage.vue` renders the final account/password form structure with a disabled submit button; Task 2 connects the same component to the authentication API without changing its route or visual boundary.

- [ ] **Step 6: Define desktop design tokens and width behavior**

Create `web/src/styles/tokens.css` with concrete CSS variables:

```css
:root {
  --memora-brand-500: #7c3aed;
  --memora-brand-600: #6d28d9;
  --memora-nav: #18181b;
  --memora-bg: #f8fafc;
  --memora-surface: #ffffff;
  --memora-border: #e4e4e7;
  --memora-text: #27272a;
  --memora-muted: #71717a;
  --memora-success: #16a34a;
  --memora-warning: #d97706;
  --memora-danger: #dc2626;
  --memora-radius-sm: 6px;
  --memora-radius-md: 10px;
  --memora-nav-width: 52px;
  --memora-sidebar-width: 260px;
  --memora-inspector-width: 360px;
}
```

Import Tailwind and the tokens from `index.css`. Do not add dark-mode tokens in P0.

Create `web/.env.example` with the complete P0 frontend environment surface:

```env
VITE_API_BASE_URL=/api/v1
VITE_MAX_UPLOAD_MB=50
VITE_SSE_RESUME_ENABLED=false
VITE_DOCUMENT_SCOPE_ENABLED=false
```

- [ ] **Step 7: Update ignore rules without discarding existing entries**

Append only these frontend entries to `.gitignore`:

```gitignore
web/node_modules/
web/dist/
web/.env
```

Do not remove or reorder unrelated user changes in `.gitignore`.

- [ ] **Step 8: Run the foundation quality gates**

Run from `web/`:

```bash
pnpm typecheck
pnpm lint
pnpm build
```

Expected: all commands exit 0 and `web/dist/` is generated. Report these as static/build results only.

- [ ] **Step 9: Commit the foundation**

```bash
git add .gitignore web
git commit -m "feat(web): scaffold Vue frontend foundation"
```

---

### Task 2: Implement API Client, Authentication, and User Session

**Files:**
- Create: `web/src/api/client.ts`
- Create: `web/src/api/errors.ts`
- Create: `web/src/api/pagination.ts`
- Create: `web/src/types/common.ts`
- Create: `web/src/stores/auth.ts`
- Create: `web/src/features/auth/api.ts`
- Create: `web/src/features/auth/types.ts`
- Modify: `web/src/features/auth/pages/LoginPage.vue`
- Create: `web/src/features/user/api.ts`
- Create: `web/src/features/user/types.ts`
- Create: `web/src/features/user/pages/ProfilePage.vue`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: `queryClient` from Task 1.
- Produces: `request<T>(path, init): Promise<T>`, `AppError`, `useAuthStore()`, `login(credentials): Promise<LoginData>`, `logout(): Promise<void>`, and authenticated route guards.

- [ ] **Step 1: Define API envelope, pagination, and error types**

Use the Shared Interfaces exactly. Add this request signature:

```ts
export interface ApiRequestInit extends Omit<RequestInit, 'body'> {
  body?: BodyInit | Record<string, unknown> | null
  auth?: boolean
}

export async function request<T>(path: string, init?: ApiRequestInit): Promise<T>
```

- [ ] **Step 2: Implement the authenticated Fetch client**

In `client.ts`, use `import.meta.env.VITE_API_BASE_URL || '/api/v1'` as the base URL, serialize plain-object bodies as JSON, preserve `FormData`, read the token through a registered getter, unwrap `ApiEnvelope<T>`, and throw `AppError` for non-2xx responses.

Use this registration boundary to avoid importing Pinia from the transport layer:

```ts
let getAccessToken: () => string | null = () => null
let onUnauthorized: () => void = () => undefined

export function configureAuthTransport(config: {
  getAccessToken: () => string | null
  onUnauthorized: () => void
}): void {
  getAccessToken = config.getAccessToken
  onUnauthorized = config.onUnauthorized
}
```

On HTTP 401, call `onUnauthorized()` once and then throw the parsed `AppError`. Export the shared parser with this exact signature for Task 8:

```ts
export async function appErrorFromResponse(response: Response): Promise<AppError>
```

- [ ] **Step 3: Implement the auth store and session persistence**

Use these exact keys and actions:

```ts
const ACCESS_TOKEN_KEY = 'memora.access_token'
const TOKEN_EXPIRES_AT_KEY = 'memora.token_expires_at'

interface AuthState {
  access_token: string | null
  token_expires_at: number | null
  user: CurrentUser | null
}

// actions: setSession, clearSession, restoreSession, isExpired
```

Store only the token and expiry timestamp in `sessionStorage`. Keep the user object in memory and reload it from `GET /users/me` after a page refresh.

- [ ] **Step 4: Implement login, logout, current-user, profile, and password APIs**

Match the documented endpoints exactly:

```ts
POST  /auth/login
POST  /auth/logout
GET   /users/me
PATCH /users/me
PATCH /users/me/password
```

Do not create refresh-token calls because the backend has no such contract.

- [ ] **Step 5: Build the login page and route guards**

The login page must contain account, password, submit state, inline validation, and a request-ID-aware error message. The router guard must:

1. allow `/login` without auth;
2. restore the session once;
3. redirect unauthenticated users to `/login?redirect=<encoded path>`;
4. load `/users/me` before entering protected routes;
5. return to the encoded redirect after login.

- [ ] **Step 6: Build profile and password forms**

Create `/settings/profile`. Never prefill the password form. After a successful password change, clear its fields and show a success notification.

- [ ] **Step 7: Verify auth behavior**

Run:

```bash
pnpm typecheck
pnpm lint
pnpm build
```

Manual checks with the Go server running:

1. wrong credentials show the backend message and `request_id`;
2. successful login redirects to the requested protected route;
3. reload restores the token and reloads the current user;
4. logout clears `sessionStorage` and returns to `/login`;
5. a forced 401 clears runtime state and redirects once.

- [ ] **Step 8: Commit authentication**

```bash
git add web/src/api web/src/stores/auth.ts web/src/features/auth web/src/features/user web/src/router
git commit -m "feat(web): add authentication and API client"
```

---

### Task 3: Build AppShell, Workspace State, and Knowledge Base Management

**Files:**
- Create: `web/src/layouts/AppShell.vue`
- Create: `web/src/layouts/UnsupportedViewport.vue`
- Create: `web/src/stores/workspace.ts`
- Create: `web/src/stores/layout.ts`
- Create: `web/src/components/base/BaseButton.vue`
- Create: `web/src/components/base/BaseDialog.vue`
- Create: `web/src/components/base/BaseDrawer.vue`
- Create: `web/src/components/base/BaseDropdown.vue`
- Create: `web/src/components/base/BaseTooltip.vue`
- Create: `web/src/components/base/BaseTabs.vue`
- Create: `web/src/components/base/BaseTree.vue`
- Create: `web/src/components/base/BaseInput.vue`
- Create: `web/src/components/base/BaseVirtualList.vue`
- Create: `web/src/components/shared/StatusBadge.vue`
- Create: `web/src/components/shared/EmptyState.vue`
- Create: `web/src/components/shared/LoadingSkeleton.vue`
- Create: `web/src/components/shared/ConfirmDialog.vue`
- Create: `web/src/components/shared/GlobalSearchDialog.vue`
- Create: `web/src/features/knowledge-base/api.ts`
- Create: `web/src/features/knowledge-base/types.ts`
- Create: `web/src/features/knowledge-base/queries.ts`
- Create: `web/src/features/knowledge-base/pages/KnowledgeBaseListPage.vue`
- Create: `web/src/features/knowledge-base/components/KnowledgeBaseCard.vue`
- Create: `web/src/features/knowledge-base/components/KnowledgeBaseFormDrawer.vue`
- Modify: `web/src/app/App.vue`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: authenticated `request<T>` from Task 2.
- Produces: `useWorkspaceStore()`, `useLayoutStore()`, `knowledgeBaseKeys`, and complete knowledge-base CRUD UI.

- [ ] **Step 1: Define knowledge-base DTOs and query keys**

Use API fields directly:

```ts
export interface KnowledgeBase {
  id: string
  name: string
  description?: string | null
  document_count?: number
  agent_enabled?: boolean
  network_enabled?: boolean
  created_at: string
  updated_at: string
}

export const knowledgeBaseKeys = {
  all: ['knowledge-bases'] as const,
  list: (query: PageQuery) => ['knowledge-bases', 'list', query] as const,
  detail: (id: string) => ['knowledge-bases', 'detail', id] as const,
}
```

- [ ] **Step 2: Implement knowledge-base CRUD and invalidation**

Use the documented endpoints and invalidate only `knowledgeBaseKeys.all` after create, update, or delete. When deleting the current knowledge base, clear `workspaceStore.current_kb_id` before navigation.

- [ ] **Step 3: Implement global shell and navigation**

The 52px dark navigation rail must contain Chat, Documents, Runs, Memories, MCP, and Settings icons with accessible labels. Put the current-user menu at the bottom. Wrap protected routes in `AppShell`.

- [ ] **Step 4: Wrap the complete base-component surface**

Wrap Reka primitives behind Memora components so feature code does not import Reka directly. `BaseTree` exposes typed `items`, `expanded_ids`, and `selected_id`; `BaseVirtualList` wraps `@tanstack/vue-virtual` and exposes item rendering through a scoped slot. Every overlay wrapper must restore focus on close.

- [ ] **Step 5: Implement the global search command**

Register `Ctrl/Cmd + K` in `AppShell` and open `GlobalSearchDialog`. In this task, the command set contains navigation and knowledge-base switching. Task 4 extends the same dialog with document lookup after the document API module exists. Do not create a separate undocumented global-search API.

- [ ] **Step 6: Implement desktop-only viewport behavior**

At widths below 1280px, render `UnsupportedViewport` instead of compressing the workspace. The notice must say that P0 requires a desktop window at least 1280px wide.

- [ ] **Step 7: Persist non-sensitive layout preferences**

Use local storage keys:

```ts
memora.layout.document_sidebar_width
memora.layout.inspector_width
memora.layout.inspector_collapsed
memora.workspace.current_kb_id
```

Clamp document sidebar width to 220–340px and inspector width to 300–480px.

- [ ] **Step 8: Build the knowledge-base list**

Implement search, pagination, cards, create/edit drawer, and delete confirmation. Show name, document count, Agent state, and updated time. Empty state must include a “创建知识库” action.

- [ ] **Step 9: Verify shell and knowledge bases**

Run typecheck, lint, and build. Manually verify 1280/1440/1920 widths, keyboard tooltips, `Ctrl/Cmd + K`, search navigation, create/edit/delete, pagination, and current knowledge-base persistence.

- [ ] **Step 10: Commit the application shell**

```bash
git add web/src/app web/src/layouts web/src/stores/workspace.ts web/src/stores/layout.ts web/src/components web/src/features/knowledge-base web/src/router
git commit -m "feat(web): add app shell and knowledge bases"
```

---

### Task 4: Implement the Notion-Style Read-Only Document Workspace

**Files:**
- Create: `web/src/layouts/DocumentWorkspaceShell.vue`
- Create: `web/src/features/document/api.ts`
- Create: `web/src/features/document/types.ts`
- Create: `web/src/features/document/queries.ts`
- Create: `web/src/features/document/pages/DocumentWorkspacePage.vue`
- Create: `web/src/features/document/components/KnowledgeTree.vue`
- Create: `web/src/features/document/components/DocumentToolbar.vue`
- Create: `web/src/features/document/components/DocumentViewer.vue`
- Create: `web/src/features/document/components/DocumentProcessingState.vue`
- Create: `web/src/features/document/components/DocumentAiPanel.vue`
- Create: `web/src/utils/markdown.ts`
- Modify: `web/src/components/shared/GlobalSearchDialog.vue`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: `current_kb_id`, authenticated request, and layout widths.
- Produces: `documentKeys`, directory/document DTOs, sanitized `renderMarkdown(source): string`, and `/kb/:kbId/docs/:documentId?`.

- [ ] **Step 1: Define directory, document, and processing types**

```ts
export interface DirectoryNode {
  id: string
  knowledge_base_id: string
  parent_id: string | null
  name: string
  sort_order: number
  children: DirectoryNode[]
}

export interface DocumentDetail {
  id: string
  knowledge_base_id: string
  directory_id: string | null
  title: string
  content: string
  source_type: 'manual' | 'file' | 'url'
  source_url: string | null
  processing_status: 'queued' | 'processing' | 'succeeded' | 'failed'
  content_version: number
  active_index_version: number | null
  created_at: string
  updated_at: string
}
```

Add explicit DTOs for list filters and processing errors from the API document; do not use `any`.

- [ ] **Step 2: Implement directory and document queries**

Use:

```text
GET    /knowledge-bases/{kb_id}/directories/tree
POST   /knowledge-bases/{kb_id}/directories
PATCH  /directories/{directory_id}
DELETE /directories/{directory_id}
GET    /knowledge-bases/{kb_id}/documents
GET    /documents/{document_id}
DELETE /documents/{document_id}
```

Invalidate the directory tree after create/move/delete. Invalidate the current knowledge-base document list after document deletion.

- [ ] **Step 3: Implement safe Markdown rendering**

Create one renderer used by both documents and chat:

```ts
import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

const markdown = new MarkdownIt({ html: false, linkify: true, breaks: true })

export function renderMarkdown(source: string): string {
  return DOMPurify.sanitize(markdown.render(source), {
    USE_PROFILES: { html: true },
  })
}
```

All external links rendered by components must receive `target="_blank"` and `rel="noopener noreferrer"` after sanitization.

- [ ] **Step 4: Build the document shell and tree**

The layout must contain the resizable 240–280px tree, centered reader, and collapsible 320–400px AI panel. Use route parameters as the source of document selection so browser back/forward works.

Extend `GlobalSearchDialog` with current-knowledge-base document lookup through the paginated document endpoint and its `keyword` parameter. Selecting a result opens `/kb/:kbId/docs/:documentId`.

- [ ] **Step 5: Build the viewer and processing states**

Behavior by status:

- `queued`: show queued status and start polling;
- `processing`: show current stage and poll every 2–3 seconds;
- `succeeded`: render sanitized `content`;
- `failed`: show failure code/message plus retry action;
- empty `content` with succeeded status: show an explicit empty-document state.

Do not embed original PDF/DOCX files because no safe preview endpoint exists.

- [ ] **Step 6: Build the read-only AI panel state**

Initially render the current document title and a scope label. Do not send questions yet. The panel will connect to conversations in Task 9. If document-scope capability is disabled, label it “基于当前知识库”.

- [ ] **Step 7: Verify the document workspace**

Run quality gates. Manually verify route selection, back/forward, nested directories, empty content, all four processing states, sanitized script payloads, long Markdown, and resizable panes.

- [ ] **Step 8: Commit the document workspace**

```bash
git add web/src/layouts/DocumentWorkspaceShell.vue web/src/features/document web/src/utils/markdown.ts web/src/router
git commit -m "feat(web): add read-only document workspace"
```

---

### Task 5: Add Document Import, Retry, Reindex, and Search Testing

**Files:**
- Create: `web/src/features/document/components/ImportDrawer.vue`
- Create: `web/src/features/document/components/ImportTaskList.vue`
- Create: `web/src/features/document/components/IndexVersionList.vue`
- Create: `web/src/features/search/api.ts`
- Create: `web/src/features/search/types.ts`
- Create: `web/src/features/search/pages/SearchTestPage.vue`
- Create: `web/src/features/search/components/RetrievalResultList.vue`
- Modify: `web/src/features/document/api.ts`
- Modify: `web/src/features/document/queries.ts`
- Modify: `web/src/features/document/pages/DocumentWorkspacePage.vue`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: document queries and API Client.
- Produces: import mutations, processing retry/reindex controls, index versions, and `/kb/:kbId/search-test`.

- [ ] **Step 1: Add import and processing APIs**

Implement these exact endpoints:

```text
POST /knowledge-bases/{kb_id}/imports/files
POST /knowledge-bases/{kb_id}/imports/url
GET  /knowledge-bases/{kb_id}/import-tasks
GET  /import-tasks/{task_id}
POST /import-tasks/{task_id}/retry
GET  /documents/{document_id}/processing
POST /documents/{document_id}/retry-processing
POST /documents/{document_id}/reindex
GET  /documents/{document_id}/index-versions
```

- [ ] **Step 2: Build file and URL import forms**

File import uses `FormData`; URL import uses JSON. Read the Task 1 `VITE_MAX_UPLOAD_MB=50` value, validate `.md`, `.txt`, `.pdf`, `.docx`, the configured size, and non-empty selection before submission, while displaying that server MIME, hash, quota, count, and size validation remains authoritative. Send `duplicate_policy` as `skip` or `create_new`, defaulting to `skip` as defined by the API.

- [ ] **Step 3: Implement finite polling**

Poll import tasks and document processing only while the status is non-terminal. Stop polling on page unmount, auth failure, success, failure, or cancellation. Never create a global interval.

- [ ] **Step 4: Build index and retry controls**

Show active index version, historical version states, reindex action, and overall processing retry. Disable actions while a conflicting task is active and surface `INDEX_VERSION_CONFLICT` inline.

- [ ] **Step 5: Implement search and retrieval test types**

Model keyword, vector, hybrid, RRF, reranker, duration, knowledge status, document/chunk identifiers, quoted text, and source location explicitly from the API response.

- [ ] **Step 6: Build the search-test page**

Allow query input and the documented search mode/config parameters. Render stage labels, ranking score, source document, quoted text, and duration. Clearly distinguish no results from `knowledge_status=insufficient`.

- [ ] **Step 7: Verify import and search**

Run quality gates. Manually verify supported file types, rejected file, URL validation, task polling stop conditions, failed retry, reindex conflict, and search result source links.

- [ ] **Step 8: Commit document operations**

```bash
git add web/src/features/document web/src/features/search web/src/router
git commit -m "feat(web): add imports and retrieval testing"
```

---

### Task 6: Implement Model, Search, and Agent Configuration

**Files:**
- Create: `web/src/features/model-config/api.ts`
- Create: `web/src/features/model-config/types.ts`
- Create: `web/src/features/model-config/queries.ts`
- Create: `web/src/features/model-config/pages/ModelConfigPage.vue`
- Create: `web/src/features/model-config/components/ModelConfigDrawer.vue`
- Create: `web/src/components/shared/SecretInput.vue`
- Create: `web/src/features/knowledge-base/pages/KnowledgeBaseSettingsPage.vue`
- Create: `web/src/features/knowledge-base/components/SearchConfigForm.vue`
- Create: `web/src/features/knowledge-base/components/AgentConfigForm.vue`
- Modify: `web/src/features/knowledge-base/api.ts`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: knowledge-base IDs and API Client.
- Produces: model CRUD, search configuration, Agent budget/configuration, and secret-safe forms.

- [ ] **Step 1: Implement model-config CRUD**

Use `/model-configs` and `/model-configs/{model_config_id}`. Define discriminated model roles for `chat`, `embedding`, and `reranker` based on the API enum. Never return secret values from form state after a successful save.

- [ ] **Step 2: Build SecretInput**

`SecretInput` accepts `configured: boolean`, `modelValue: string`, and `mode: 'create' | 'replace'`. It displays only “已配置” for existing credentials and never receives a masked backend secret as a usable value.

- [ ] **Step 3: Implement search configuration**

Use `GET/PUT /knowledge-bases/{kb_id}/search-config`. Bind the exact fields `keyword_top_k`, `vector_top_k`, `rrf_k`, `rrf_top_k`, `reranker_top_k`, `reranker_threshold`, `minimum_effective_results`, and `reranker_model_id`. Require top-k/count fields to be positive integers and `reranker_threshold` to be within 0–1; treat any tighter backend range returned as `INVALID_ARGUMENT` as a field error. Do not invent replacement defaults when the GET response supplies values.

- [ ] **Step 4: Implement Agent configuration**

Use `GET/PUT /knowledge-bases/{kb_id}/agent-config`. Include `name`, `system_prompt`, `chat_model_id`, `max_react_rounds`, `max_plan_steps`, `max_replans`, `reviewer_runs`, `max_tool_calls`, `max_document_read_tokens`, `max_tool_result_bytes`, `max_run_seconds`, `network_enabled`, `memory_enabled`, `memory_top_k`, `show_execution_status`, and `status`. Enforce `max_react_rounds <= 8`, `max_plan_steps <= 5`, `max_replans <= 1`, `reviewer_runs <= 1`, and `max_tool_calls <= 10`.

Do not add an execution-mode selector.

- [ ] **Step 5: Build settings routes and forms**

Create `/settings/models` and `/kb/:kbId/settings`. Group Agent settings into Model, Budget, Network, Memory, and Tool Authorization sections. MCP tool authorization controls remain disabled until Task 12 provides tool data.

- [ ] **Step 6: Verify configuration safety**

Run quality gates. Manually verify create/update/delete, secret replacement, numeric validation, no secret in DOM after save, no execution mode input, and correct query invalidation.

- [ ] **Step 7: Commit configuration pages**

```bash
git add web/src/features/model-config web/src/features/knowledge-base web/src/components/shared/SecretInput.vue web/src/router
git commit -m "feat(web): add model and agent configuration"
```

---

### Task 7: Build Conversation and Chat Workspace Foundations

**Files:**
- Create: `web/src/layouts/ChatWorkspaceShell.vue`
- Create: `web/src/features/conversation/api.ts`
- Create: `web/src/features/conversation/types.ts`
- Create: `web/src/features/conversation/queries.ts`
- Create: `web/src/features/conversation/pages/ChatPage.vue`
- Create: `web/src/features/conversation/components/ConversationSidebar.vue`
- Create: `web/src/features/conversation/components/MessageList.vue`
- Create: `web/src/features/conversation/components/UserMessage.vue`
- Create: `web/src/features/conversation/components/AssistantMessage.vue`
- Create: `web/src/features/conversation/components/ChatComposer.vue`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: current knowledge base, `renderMarkdown`, and Vue Query.
- Produces: conversation/message queries, `submitQuestion(conversationId, input): Promise<QuestionAccepted>`, and `/chat/:kbId/:conversationId?`.

- [ ] **Step 1: Define conversation and message DTOs**

```ts
export interface Conversation {
  id: string
  knowledge_base_id: string
  title: string
  created_at: string
}

export type Citation = KnowledgeBaseCitation | NetworkCitation

export interface KnowledgeBaseCitation {
  source_type: 'knowledge_base'
  document_id: string
  document_title: string
  chunk_id: string
  quoted_text: string
  knowledge_base_id: string
  source_location: { section?: string; page?: number }
  document_updated_at: string
}

export interface NetworkCitation {
  source_type: 'network'
  title: string
  url: string
  site_name: string
  published_at: string | null
  fetched_at: string
}

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  agent_run_id: string | null
  status?: 'streaming' | 'completed' | 'failed' | 'cancelled'
  citations?: Citation[]
  created_at: string
}

export interface QuestionAccepted {
  run_id: string
  user_message_id: string
  status: 'queued'
  events_url: string
}
```

- [ ] **Step 2: Implement conversation and question APIs**

Use create/list/detail/messages/delete/question endpoints exactly. The question input type at this stage is:

```ts
export interface QuestionInput {
  query: string
  document_id?: string
}
```

Only include `document_id` when the capability flag described in Task 9 is enabled.

- [ ] **Step 3: Build the chat shell and conversation sidebar**

Use a 240–280px conversation sidebar, centered message area, and a stable inspector host that renders `EmptyState` with “提交问题后显示执行过程” when no run is active. Task 8 connects the inspector host to runtime data without replacing the layout boundary. Support search, create, select, and delete. Route selection is the source of truth.

- [ ] **Step 4: Build message rendering and composer**

Render assistant Markdown through `renderMarkdown`. The composer supports multiline input, submit, disabled states, and `Ctrl/Cmd + Enter`. It must not show any execution-mode control.

- [ ] **Step 5: Implement accepted-question optimistic state**

After HTTP 202:

1. add the user message to the visible query cache;
2. add one temporary assistant message with `status='streaming'`;
3. store `run_id` and `events_url` for Task 8;
4. disable duplicate submission for the active conversation.

- [ ] **Step 6: Verify chat foundations**

Run quality gates. Manually verify conversation routing, paging, Markdown sanitization, no mode selector, accepted-question temporary messages, and duplicate-submit prevention.

- [ ] **Step 7: Commit chat foundations**

```bash
git add web/src/layouts/ChatWorkspaceShell.vue web/src/features/conversation web/src/router
git commit -m "feat(web): add conversation workspace"
```

---

### Task 8: Implement Authenticated SSE and the Agent Runtime Panel

**Files:**
- Create: `web/src/api/sse.ts`
- Create: `web/src/features/agent-run/events.ts`
- Create: `web/src/features/agent-run/types.ts`
- Create: `web/src/stores/agent-runtime.ts`
- Create: `web/src/features/agent-run/components/AgentRunPanel.vue`
- Create: `web/src/features/agent-run/components/RouterSummary.vue`
- Create: `web/src/features/agent-run/components/PlanTimeline.vue`
- Create: `web/src/features/agent-run/components/ReactRounds.vue`
- Create: `web/src/features/agent-run/components/ToolCallList.vue`
- Create: `web/src/features/agent-run/components/UsageSummary.vue`
- Modify: `web/src/features/conversation/pages/ChatPage.vue`
- Modify: `web/src/features/conversation/components/AssistantMessage.vue`
- Modify: `web/src/features/conversation/components/ChatComposer.vue`

**Interfaces:**
- Consumes: `QuestionAccepted`, auth token, and message query cache.
- Produces: `streamAgentEvents(options): Promise<void>`, typed `KnownAgentEvent` union, runtime projection, stop/retry controls, and a complete live inspector.

- [ ] **Step 1: Define the exact event union**

Create a discriminated union for every required event:

```ts
export interface AgentEventBase<TName extends string, TPayload> {
  event: TName
  run_id: string
  sequence: number
  timestamp: string
  payload: TPayload
}
```

Use these concrete payloads and define the complete known-event union:

```ts
export interface PlanEventPayload {
  plan_id: string
  version: 1 | 2
  goal: string
  replan_reason?: string
}

export interface StepEventPayload {
  step_id: string
  step_no: number
  title: string
  output_summary?: string
  error_message?: string
}

export interface RoundEventPayload {
  round_no: number
  action_summary?: string
}

export interface ToolCallEventPayload {
  tool_call_id: string
  tool_name: string
  tool_type: 'internal' | 'mcp'
  input_summary?: string
  output_summary?: string
  duration_ms?: number
  is_truncated?: boolean
  error_message?: string
}

export interface MemoryUpdatedPayload {
  memory_id?: string
  action: 'created' | 'merged' | 'updated' | 'invalidated'
}

export type KnownAgentEvent =
  | AgentEventBase<'run.started', Record<string, never>>
  | AgentEventBase<'run.completed', Record<string, unknown>>
  | AgentEventBase<'run.failed', { code: string; message: string }>
  | AgentEventBase<'run.cancelled', Record<string, never>>
  | AgentEventBase<'router.selected', { execution_mode: 'react' | 'plan_execute'; reason_summary: string }>
  | AgentEventBase<'memory.retrieved', { count: number }>
  | AgentEventBase<'plan.created', PlanEventPayload>
  | AgentEventBase<'plan.replanned', PlanEventPayload>
  | AgentEventBase<'step.started', StepEventPayload>
  | AgentEventBase<'step.completed', StepEventPayload>
  | AgentEventBase<'step.failed', StepEventPayload>
  | AgentEventBase<'agent.round.started', RoundEventPayload>
  | AgentEventBase<'tool.call.started', ToolCallEventPayload>
  | AgentEventBase<'tool.call.completed', ToolCallEventPayload>
  | AgentEventBase<'tool.call.failed', ToolCallEventPayload>
  | AgentEventBase<'answer.delta', { delta: string }>
  | AgentEventBase<'citation.created', { citation: Citation }>
  | AgentEventBase<'usage.updated', { input_tokens: number; output_tokens: number; total_tokens: number }>
  | AgentEventBase<'memory.updated', MemoryUpdatedPayload>
```

Import `Citation` from `features/conversation/types.ts`. Parse forward-compatible events as a separate `UnknownAgentEvent` and never send that type to UI projection:

```ts
export interface UnknownAgentEvent extends AgentEventBase<string, Record<string, unknown>> {
  unknown: true
}

export type ParsedAgentEvent = KnownAgentEvent | UnknownAgentEvent
```

- [ ] **Step 2: Implement the authenticated SSE parser**

Use Fetch so the request can carry the Bearer header:

```ts
export interface StreamAgentEventsOptions {
  url: string
  access_token: string
  signal: AbortSignal
  after_sequence?: number
  on_event: (event: ParsedAgentEvent) => void
}

export async function streamAgentEvents(options: StreamAgentEventsOptions): Promise<void> {
  const url = new URL(options.url, window.location.origin)
  if (options.after_sequence !== undefined) {
    url.searchParams.set('after_sequence', String(options.after_sequence))
  }

  const response = await fetch(url, {
    headers: { Authorization: `Bearer ${options.access_token}`, Accept: 'text/event-stream' },
    signal: options.signal,
  })
  if (!response.ok || !response.body) throw await appErrorFromResponse(response)

  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader()
  let buffer = ''
  while (true) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += value.replace(/\r\n/g, '\n')
    let boundary = buffer.indexOf('\n\n')
    while (boundary >= 0) {
      const block = buffer.slice(0, boundary)
      buffer = buffer.slice(boundary + 2)
      const parsed = parseSseBlock(block)
      if (parsed) options.on_event(parsed)
      boundary = buffer.indexOf('\n\n')
    }
  }
}
```

In `events.ts`, define the parser used above. Preserve multiple `data:` lines and ignore SSE comments:

```ts
const baseEventSchema = z.object({
  run_id: z.string().uuid(),
  sequence: z.number().int().nonnegative(),
  timestamp: z.string(),
  payload: z.record(z.string(), z.unknown()),
})

export function parseSseBlock(block: string): ParsedAgentEvent | null {
  let eventName = 'message'
  const dataLines: string[] = []
  for (const line of block.split('\n')) {
    if (!line || line.startsWith(':')) continue
    const separator = line.indexOf(':')
    const field = separator < 0 ? line : line.slice(0, separator)
    const rawValue = separator < 0 ? '' : line.slice(separator + 1)
    const value = rawValue.startsWith(' ') ? rawValue.slice(1) : rawValue
    if (field === 'event') eventName = value
    if (field === 'data') dataLines.push(value)
  }
  if (dataLines.length === 0) return null
  return parseAgentEvent(eventName, JSON.parse(dataLines.join('\n')))
}

export function parseAgentEvent(eventName: string, raw: unknown): ParsedAgentEvent {
  const base = baseEventSchema.parse(raw)
  const parser = knownPayloadSchemas[eventName as keyof typeof knownPayloadSchemas]
  if (!parser) return { ...base, event: eventName, unknown: true }
  return { ...base, event: eventName, payload: parser.parse(base.payload) } as KnownAgentEvent
}
```

Define `knownPayloadSchemas` in the same file:

```ts
const recordPayload = z.record(z.string(), z.unknown())
const planPayload = z.object({
  plan_id: z.string().uuid(),
  version: z.union([z.literal(1), z.literal(2)]),
  goal: z.string(),
  replan_reason: z.string().optional(),
})
const stepPayload = z.object({
  step_id: z.string().uuid(),
  step_no: z.number().int().positive(),
  title: z.string(),
  output_summary: z.string().optional(),
  error_message: z.string().optional(),
})
const toolPayload = z.object({
  tool_call_id: z.string().uuid(),
  tool_name: z.string(),
  tool_type: z.enum(['internal', 'mcp']),
  input_summary: z.string().optional(),
  output_summary: z.string().optional(),
  duration_ms: z.number().nonnegative().optional(),
  is_truncated: z.boolean().optional(),
  error_message: z.string().optional(),
})
const knowledgeCitation = z.object({
  source_type: z.literal('knowledge_base'),
  document_id: z.string().uuid(),
  document_title: z.string(),
  chunk_id: z.string().uuid(),
  quoted_text: z.string(),
  knowledge_base_id: z.string().uuid(),
  source_location: z.object({ section: z.string().optional(), page: z.number().int().positive().optional() }),
  document_updated_at: z.string(),
})
const networkCitation = z.object({
  source_type: z.literal('network'),
  title: z.string(),
  url: z.string().url(),
  site_name: z.string(),
  published_at: z.string().nullable(),
  fetched_at: z.string(),
})

export const knownPayloadSchemas = {
  'run.started': recordPayload,
  'run.completed': recordPayload,
  'run.failed': z.object({ code: z.string(), message: z.string() }),
  'run.cancelled': recordPayload,
  'router.selected': z.object({ execution_mode: z.enum(['react', 'plan_execute']), reason_summary: z.string() }),
  'memory.retrieved': z.object({ count: z.number().int().nonnegative() }),
  'plan.created': planPayload,
  'plan.replanned': planPayload,
  'step.started': stepPayload,
  'step.completed': stepPayload,
  'step.failed': stepPayload,
  'agent.round.started': z.object({ round_no: z.number().int().positive(), action_summary: z.string().optional() }),
  'tool.call.started': toolPayload,
  'tool.call.completed': toolPayload,
  'tool.call.failed': toolPayload,
  'answer.delta': z.object({ delta: z.string() }),
  'citation.created': z.object({ citation: z.union([knowledgeCitation, networkCitation]) }),
  'usage.updated': z.object({ input_tokens: z.number().int().nonnegative(), output_tokens: z.number().int().nonnegative(), total_tokens: z.number().int().nonnegative() }),
  'memory.updated': z.object({ memory_id: z.string().uuid().optional(), action: z.enum(['created', 'merged', 'updated', 'invalidated']) }),
} as const
```

A schema failure is a malformed stream event: stop projection, reconcile the persisted AgentRun, and display the connection error without rendering the raw payload.

Export `appErrorFromResponse` from `client.ts` rather than duplicating error parsing.

- [ ] **Step 3: Implement deterministic runtime projection**

The runtime store must contain `run_id`, `status`, `last_sequence`, `execution_mode`, `router_reason_summary`, `answer`, `citations`, `plan_versions`, `steps`, `rounds`, `tool_calls`, `usage`, and `error`.

Ignore events where `sequence <= last_sequence`. Append only `answer.delta.payload.delta`. Stop the stream on completed, failed, cancelled, route leave, or logout.

- [ ] **Step 4: Implement cancel and retry**

Use:

```text
POST /agent-runs/{run_id}/cancel
POST /agent-runs/{run_id}/retry
```

Cancel aborts the local stream after the backend acknowledges cancellation. Retry resets runtime state using the returned `new_run_id` and subscribes to the new events URL derived from that ID.

- [ ] **Step 5: Implement disconnect reconciliation**

Use the Task 1 default `VITE_SSE_RESUME_ENABLED=false`. When false, a non-terminal disconnect triggers `GET /agent-runs/{run_id}`:

- terminal response: refresh messages and render the persisted result;
- running response: show a reconnect action;
- reconnect: subscribe without `after_sequence` and continue deduplicating by sequence.

When the backend contract is confirmed, setting `VITE_SSE_RESUME_ENABLED=true` passes `last_sequence` as `after_sequence`.

- [ ] **Step 6: Build the runtime inspector**

Render Router summary, current Plan/steps or ReAct rounds, tool calls, usage, duration, and final status. Collapse detailed execution by default. Never render unknown event payloads as raw JSON in the user interface.

- [ ] **Step 7: Verify the SSE matrix**

Run quality gates. Manually exercise: normal ReAct completion, Plan completion, tool failure, run failure, cancel, retry, duplicate sequence, malformed event, network disconnect, 401 during stream, and route navigation cleanup.

- [ ] **Step 8: Commit streaming Agent chat**

```bash
git add web/src/api web/src/stores/agent-runtime.ts web/src/features/agent-run web/src/features/conversation web/.env.example
git commit -m "feat(web): add streaming Agent runtime"
```

---

### Task 9: Add Citations, Document Navigation, and Current-Document AI

**Files:**
- Create: `web/src/features/agent-run/components/CitationPopover.vue`
- Create: `web/src/features/agent-run/components/CitationList.vue`
- Create: `web/src/features/document/components/CitationHighlight.vue`
- Create: `web/src/features/document/composables/useDocumentScopeChat.ts`
- Modify: `web/src/features/conversation/components/AssistantMessage.vue`
- Modify: `web/src/features/document/components/DocumentViewer.vue`
- Modify: `web/src/features/document/components/DocumentAiPanel.vue`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: citations from messages/events, document routes, and question submission.
- Produces: source-aware citations, document section/text highlighting, and document-scoped questions with explicit fallback.

- [ ] **Step 1: Reuse the citation types defined by Task 7**

Import `Citation`, `KnowledgeBaseCitation`, and `NetworkCitation` from `features/conversation/types.ts`. Do not create a second citation union in the Agent or Document features.

- [ ] **Step 2: Render source-aware citations**

Knowledge citations open an internal document route. Network citations open a sanitized external URL in a new tab with `noopener noreferrer`. Use different icons and labels; do not style Memory as a factual citation.

- [ ] **Step 3: Implement document location state**

Use query parameters:

```text
/kb/{kbId}/docs/{documentId}?section=<encoded>&quote=<encoded>&fromConversation=<id>
```

On load, find the first matching section, then the first normalized occurrence of `quoted_text`. Scroll it into view and highlight it. If text cannot be matched, scroll to the section and show “引用文本可能来自旧版本”.

- [ ] **Step 4: Implement the current-document capability flag**

Use the Task 1 environment value:

```env
VITE_DOCUMENT_SCOPE_ENABLED=false
```

When false, do not send `document_id`; label the panel “基于当前知识库”. When true, include the current document ID and label it “基于当前文档”. Do not fake scope by injecting document text into the user query.

- [ ] **Step 5: Connect DocumentAiPanel to conversations**

Reuse or create a conversation in the current knowledge base. Submit through the same question endpoint and open the chat/runtime projection in the right panel. Preserve a link to open the full chat workspace.

- [ ] **Step 6: Verify citations and scope**

Run quality gates. Manually verify knowledge/network distinction, external-link attributes, current/old document highlighting, return-to-conversation navigation, capability false fallback, and capability true request body.

- [ ] **Step 7: Commit citations and document AI**

```bash
git add web/src/features/agent-run web/src/features/document web/src/features/conversation web/src/router web/.env.example
git commit -m "feat(web): add citations and document AI"
```

---

### Task 10: Implement Agent Run History and Details

**Files:**
- Create: `web/src/features/agent-run/api.ts`
- Create: `web/src/features/agent-run/queries.ts`
- Create: `web/src/features/agent-run/pages/AgentRunListPage.vue`
- Create: `web/src/features/agent-run/pages/AgentRunDetailPage.vue`
- Create: `web/src/features/agent-run/components/RunFilters.vue`
- Create: `web/src/features/agent-run/components/RunSummaryCard.vue`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: existing Agent types and display components.
- Produces: persisted run list/detail queries and `/runs`, `/runs/:runId`.

- [ ] **Step 1: Implement run APIs**

Use:

```text
GET /agent-runs
GET /agent-runs/{run_id}
GET /agent-runs/{run_id}/router-decision
GET /agent-runs/{run_id}/plans
GET /agent-runs/{run_id}/rounds
GET /agent-runs/{run_id}/tool-calls
GET /agent-runs/{run_id}/citations
GET /agent-runs/{run_id}/memories
```

Reuse Task 8 types. Do not define a second incompatible PlanStep or ToolCall shape.

- [ ] **Step 2: Build run filters and paginated list**

Support documented filters for status, knowledge base, execution mode, time, keyword, and sort. Keep filters in URL query parameters so the view is shareable and browser navigation works.

- [ ] **Step 3: Build the detail composition**

Load summary first, then fetch mode-specific details. Show Router reason, current Plan and optional version 1/2, ReAct rounds, tools, citations, Memory count, tokens, duration, result, and error. Do not show a Plan version comparison tool because it is outside P0.

- [ ] **Step 4: Verify run observability**

Run quality gates. Manually verify React/Plan modes, failed/cancelled runs, missing optional fields, citations, pagination, filter URL restoration, and absence of hidden reasoning.

- [ ] **Step 5: Commit run history**

```bash
git add web/src/features/agent-run web/src/router
git commit -m "feat(web): add Agent run history"
```

---

### Task 11: Implement Long-Term Memory Management

**Files:**
- Create: `web/src/features/memory/api.ts`
- Create: `web/src/features/memory/types.ts`
- Create: `web/src/features/memory/queries.ts`
- Create: `web/src/features/memory/pages/MemoryPage.vue`
- Create: `web/src/features/memory/components/MemoryList.vue`
- Create: `web/src/features/memory/components/MemoryDetailDrawer.vue`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: API Client and pagination.
- Produces: Memory list/detail/status/delete and `/memories`.

- [ ] **Step 1: Define Memory DTOs and filters**

Model exact enums for type, scope, and status. Include source conversation/message, created/updated/last-used times, and knowledge-base scope. Do not add an editable content field to form state.

- [ ] **Step 2: Implement Memory APIs**

Use:

```text
GET    /memories
GET    /memories/{memory_id}
PATCH  /memories/{memory_id}/status
DELETE /memories/{memory_id}
```

Status mutation accepts only `active` or `inactive` from the UI.

- [ ] **Step 3: Build list and detail drawer**

Support status, type, scope, knowledge-base, keyword, and sort filters defined by the API. Show source navigation when source IDs are present. Provide activate, deactivate, and delete; do not provide content editing.

- [ ] **Step 4: Verify Memory behavior**

Run quality gates. Manually verify filters, source links, inactive state, delete confirmation, empty state, forbidden access, and no edit control.

- [ ] **Step 5: Commit Memory management**

```bash
git add web/src/features/memory web/src/router
git commit -m "feat(web): add Memory management"
```

---

### Task 12: Implement MCP Server and Read-Only Tool Management

**Files:**
- Create: `web/src/features/mcp/api.ts`
- Create: `web/src/features/mcp/types.ts`
- Create: `web/src/features/mcp/queries.ts`
- Create: `web/src/features/mcp/pages/McpPage.vue`
- Create: `web/src/features/mcp/components/McpServerCard.vue`
- Create: `web/src/features/mcp/components/McpServerDrawer.vue`
- Create: `web/src/features/mcp/components/McpToolList.vue`
- Create: `web/src/features/mcp/components/ToolSchemaDrawer.vue`
- Modify: `web/src/features/knowledge-base/components/AgentConfigForm.vue`
- Modify: `web/src/router/index.ts`

**Interfaces:**
- Consumes: `SecretInput`, knowledge-base Agent config, API Client.
- Produces: MCP Server CRUD, connection test, discovery, tool enablement, schema display, and knowledge-base grants.

- [ ] **Step 1: Define MCP types with read-only safety**

Model Server status, transport type, credential configured state, tool schema, tool status, `read_only`, and grant state. The UI type for transport permits only `streamable_http`.

- [ ] **Step 2: Implement MCP endpoints**

Implement these exact endpoints. Keep Server-level enable/disable separate from knowledge-base authorization and parse `WRITE_MCP_TOOL_FORBIDDEN` as a non-retryable safety error.

```text
POST   /mcp/servers
GET    /mcp/servers
GET    /mcp/servers/{server_id}
PATCH  /mcp/servers/{server_id}
DELETE /mcp/servers/{server_id}
POST   /mcp/servers/{server_id}/test
POST   /mcp/servers/{server_id}/discover
GET    /mcp/servers/{server_id}/tools
GET    /mcp/tools/{tool_id}
PATCH  /mcp/tools/{tool_id}/enabled
PUT    /knowledge-bases/{kb_id}/agent-config/mcp-tools/{tool_id}
DELETE /knowledge-bases/{kb_id}/agent-config/mcp-tools/{tool_id}
GET    /tool-calls?tool_type=mcp
```

- [ ] **Step 3: Build Server cards and configuration drawer**

Support URL, request headers, credentials, test, discover, update, and delete. Existing credentials show only configured state. The form must not expose command, stdio, local process, or human-confirmation fields.

- [ ] **Step 4: Build tool list and schema drawer**

Show tool name, description, read-only state, enabled state, schema hash, and JSON Schema. Render schema as escaped text or structured tree, never unsanitized HTML.

- [ ] **Step 5: Connect knowledge-base tool authorization**

Populate the Agent settings tool section with enabled, read-only tools. Grant through PUT and revoke through DELETE. Disable grant controls for write tools even if a malformed API response marks them enabled.

- [ ] **Step 6: Verify MCP safety and failures**

Run quality gates. Manually verify connection success/failure/timeout, discovery, secret redaction, Server disable, grant/revoke, write-tool rejection, and an unavailable MCP Server not blocking knowledge-base chat.

- [ ] **Step 7: Commit MCP management**

```bash
git add web/src/features/mcp web/src/features/knowledge-base/components/AgentConfigForm.vue web/src/router
git commit -m "feat(web): add read-only MCP management"
```

---

### Task 13: Harden Error Handling, Accessibility, and Sensitive Data Boundaries

**Files:**
- Create: `web/src/components/shared/ErrorState.vue`
- Create: `web/src/components/shared/GlobalNotificationHost.vue`
- Create: `web/src/components/shared/RequestIdCopy.vue`
- Create: `web/src/utils/format.ts`
- Create: `web/src/utils/redact.ts`
- Modify: `web/src/api/client.ts`
- Modify: `web/src/app/App.vue`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/features/auth/pages/LoginPage.vue`
- Modify: `web/src/features/user/pages/ProfilePage.vue`
- Modify: `web/src/features/knowledge-base/pages/KnowledgeBaseListPage.vue`
- Modify: `web/src/features/knowledge-base/pages/KnowledgeBaseSettingsPage.vue`
- Modify: `web/src/features/document/pages/DocumentWorkspacePage.vue`
- Modify: `web/src/features/search/pages/SearchTestPage.vue`
- Modify: `web/src/features/model-config/pages/ModelConfigPage.vue`
- Modify: `web/src/features/conversation/pages/ChatPage.vue`
- Modify: `web/src/features/agent-run/pages/AgentRunListPage.vue`
- Modify: `web/src/features/agent-run/pages/AgentRunDetailPage.vue`
- Modify: `web/src/features/memory/pages/MemoryPage.vue`
- Modify: `web/src/features/mcp/pages/McpPage.vue`

**Interfaces:**
- Consumes: `AppError` and all pages.
- Produces: consistent inline, toast, page-level, and auth errors; request-ID diagnostics; redaction helpers; keyboard-complete UI.

- [ ] **Step 1: Define error presentation rules in code**

Create:

```ts
export type ErrorPresentation = 'field' | 'notification' | 'page' | 'auth'

export function classifyError(error: AppError): ErrorPresentation {
  if (error.code === 'UNAUTHORIZED') return 'auth'
  if (error.httpStatus === 404 || error.httpStatus === 403) return 'page'
  if (error.code === 'INVALID_ARGUMENT') return 'field'
  return 'notification'
}
```

Add explicit user messages for rate limiting, model failure, MCP failure, upstream timeout, file size/type, invalid state, and index conflict.

- [ ] **Step 2: Implement safe redaction**

`redact.ts` must redact object keys matching `password`, `token`, `secret`, `api_key`, `authorization`, and `cookie`, case-insensitively. Use it before any development diagnostic output.

- [ ] **Step 3: Add route and page error boundaries**

Every route page must have loading, empty, error, and ready states. `ErrorState` shows a retry only for retryable errors and a copyable `request_id`. It must not display `details` for HTTP 500 responses.

- [ ] **Step 4: Complete accessibility pass**

Verify:

- every icon-only button has an accessible name;
- dialogs trap focus and restore it on close;
- menus and trees work by keyboard;
- status has text in addition to color;
- visible focus indicators are not removed;
- external links announce that they open a new tab;
- form labels and error descriptions are programmatically associated.

- [ ] **Step 5: Verify security payloads**

Run quality gates. Manually inject Markdown containing `<script>`, `javascript:` links, event handlers, iframe, SVG payloads, and malformed HTML. Confirm none execute. Inspect DOM/session storage/network logs to confirm secrets are not rendered after save.

- [ ] **Step 6: Commit hardening**

```bash
git add web/src
git commit -m "fix(web): harden errors and sensitive data handling"
```

---

### Task 14: Add Production Delivery, Documentation, and P0 Acceptance

**Files:**
- Create: `web/nginx.conf`
- Create: `web/Dockerfile`
- Create: `web/README.md`
- Create: `docs/FRONTEND.md`
- Modify: `deploy/docker-compose.yml`
- Modify: `README.md`
- Modify: `.env.example`

**Interfaces:**
- Consumes: the complete SPA and current Docker deployment.
- Produces: production static hosting, same-origin API/SSE proxy, developer instructions, and a completed P0 acceptance record.

- [ ] **Step 1: Create the frontend production image**

Use a multi-stage Dockerfile:

```dockerfile
FROM node:24.18.0-alpine AS build
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM nginx:1.30.4-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
```

If project policy requires different pinned base images, update both image references and document the reason in the same commit.

- [ ] **Step 2: Configure SPA, API proxy, and SSE**

Create `web/nginx.conf` with these required behaviors:

```nginx
location / {
    try_files $uri $uri/ /index.html;
}

location /api/v1/agent-runs/ {
    proxy_pass http://api:8080;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 1h;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}

location /api/ {
    proxy_pass http://api:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

The current Compose service name is exactly `api`; use `api:8080` for API, SSE, and health proxy locations.

- [ ] **Step 3: Add the frontend service to Compose**

Build from `web/`, expose the configured Web port, and depend on the API health check. Do not merge the Vue application into the Go server process.

- [ ] **Step 4: Document local development and environment variables**

Document:

```text
pnpm install
pnpm dev
pnpm typecheck
pnpm lint
pnpm build
```

Explain `VITE_API_BASE_URL`, `VITE_SSE_RESUME_ENABLED`, and `VITE_DOCUMENT_SCOPE_ENABLED`, including the backend contract required before enabling the last two flags.

- [ ] **Step 5: Run final static/build verification**

Run from `web/`:

```bash
pnpm install --frozen-lockfile
pnpm typecheck
pnpm lint
pnpm build
```

Run from the repository root:

```bash
docker compose --env-file .env -f deploy/docker-compose.yml config
```

Expected: all commands exit 0. This does not prove runtime API acceptance.

- [ ] **Step 6: Execute the documented P0 manual chain**

With all services running, complete in order:

```text
login
→ create knowledge base
→ upload supported file
→ observe import and processing terminal state
→ run retrieval test
→ create conversation
→ submit question
→ receive SSE events and final answer
→ open a knowledge citation in the document workspace
→ inspect AgentRun detail
→ inspect Memory list
```

Then complete the MCP/network chain:

```text
create MCP Server
→ connection test
→ discover tools
→ enable a read-only tool
→ grant it to the knowledge base
→ enable networking
→ ask a current-information question
→ inspect tool calls and network citations
```

Record each result as pass/fail with `request_id` for failures. Do not label skipped backend endpoints as frontend passes.

- [ ] **Step 7: Complete visual and failure-state acceptance**

Check 1280px, 1440px, and 1920px widths. Exercise empty, loading, forbidden, not found, rate limited, model failure, MCP failure, SSE disconnect, cancelled run, failed document, long document, and long conversation states.

- [ ] **Step 8: Commit delivery files**

```bash
git add web/Dockerfile web/nginx.conf web/README.md docs/FRONTEND.md deploy/docker-compose.yml README.md .env.example
git commit -m "build(web): add frontend delivery and documentation"
```

---

### Task 15: Repair App Visibility, Visual System, Runtime Recovery, and Document AI

**Files:**
- Modify: `web/src/styles/tokens.css`
- Modify: `web/src/styles/index.css`
- Modify: `web/src/layouts/AppShell.vue`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/components/shared/EmptyState.vue`
- Modify: `web/src/components/shared/ErrorState.vue`
- Modify: `web/src/components/shared/LoadingSkeleton.vue`
- Modify: `web/src/features/knowledge-base/pages/KnowledgeBaseListPage.vue`
- Modify: `web/src/features/conversation/api.ts`
- Modify: `web/src/features/conversation/queries.ts`
- Modify: `web/src/features/conversation/pages/ChatPage.vue`
- Create: `web/src/features/agent-run/composables/useAgentRunStream.ts`
- Modify: `web/src/features/agent-run/api.ts`
- Modify: `web/src/features/agent-run/queries.ts`
- Modify: `web/src/features/agent-run/types.ts`
- Modify: `web/src/features/agent-run/components/AgentRunPanel.vue`
- Modify: `web/src/features/agent-run/components/CitationPopover.vue`
- Modify: `web/src/features/document/components/DocumentAiPanel.vue`
- Modify: `web/src/features/document/components/CitationHighlight.vue`
- Modify: `web/src/features/document/pages/DocumentWorkspacePage.vue`
- Modify: `web/nginx.conf`
- Modify: `web/Dockerfile`
- Add: `web/.gitignore`
- Add: `web/pnpm-workspace.yaml`
- Modify: `web/README.md`
- Modify: `docs/FRONTEND.md`

**Interfaces:**
- Consumes: the existing API envelope, `QuestionAccepted`, AgentRun SSE events, `VITE_SSE_RESUME_ENABLED`, `VITE_DOCUMENT_SCOPE_ENABLED`, Vue Router route metadata, and the Compose service named `api`.
- Produces: visible nested routes, a fixed visual token contract, `submitQuestion({ conversation_id, question })`, `cancelAgentRun(runId)`, `retryAgentRun(runId)`, reusable AgentRun stream lifecycle management, functional document-side chat, navigable citations, and reproducible delivery files.

- [ ] **Step 1: Restore every routed page inside AppShell**

Replace the AppShell content placeholder with Vue Router's child outlet:

```vue
<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<main class="min-w-0 flex-1 overflow-hidden">
  <RouterView />
</main>
```

Do not use `<slot />` for route children. Nest `/chat/:kbId/:conversationId?` under `AppShell` exactly like the other protected workspaces so chat never loses the global navigation.

Make navigation destinations knowledge-base-aware:

```ts
function resolveWorkspacePath(kind: 'chat' | 'docs'): string {
  const kbId = workspaceStore.current_kb_id
  if (!kbId) return '/knowledge-bases'
  return kind === 'chat' ? `/chat/${kbId}` : `/kb/${kbId}/docs`
}
```

The logo routes to `/knowledge-bases`. The Chat and Document buttons use `resolveWorkspacePath`; no button may navigate to unmatched `/chat` or `/kb` routes.

- [ ] **Step 2: Freeze the visual token and icon contract**

Define these exact tokens in `tokens.css` and use them instead of scattered color literals:

```css
:root {
  --memora-brand-50: #f5f3ff;
  --memora-brand-500: #7c3aed;
  --memora-brand-600: #6d28d9;
  --memora-focus: #8b5cf6;
  --memora-nav: #18181b;
  --memora-nav-muted: #a1a1aa;
  --memora-bg: #f8fafc;
  --memora-surface: #ffffff;
  --memora-surface-subtle: #fafafa;
  --memora-border: #e4e4e7;
  --memora-text-strong: #18181b;
  --memora-text: #27272a;
  --memora-muted: #71717a;
  --memora-info: #2563eb;
  --memora-success: #16a34a;
  --memora-warning: #d97706;
  --memora-danger: #dc2626;
  --memora-nav-width: 60px;
  --memora-header-height: 64px;
  --memora-sidebar-width: 260px;
  --memora-inspector-width: 360px;
  --memora-radius-control: 6px;
  --memora-radius-panel: 10px;
}
```

Set the application font stack to `Inter, "Noto Sans SC", "Microsoft YaHei", system-ui, sans-serif`. Use the exact typography and spacing rules in design section 12.2. Add a shared `:focus-visible` ring using `--memora-focus` and do not remove outlines without an equivalent replacement.

Replace AppShell's inline placeholder SVGs with named Lucide components. Use `22px` navigation icons with `stroke-width="1.75"`, a `40×40px` hit area, `--memora-nav-muted` at rest, white on hover, and white on a brand-colored active background. Keep visible tooltips and accessible names.

- [ ] **Step 3: Make loading, empty, error, and ready states unmistakable**

Every page must keep a `64px` header visible while its query changes. For the knowledge-base page, destructure and render the complete query state:

```ts
const { data, isLoading, isError, error, refetch } = useKnowledgeBaseList(query)
```

Render in this strict order:

```vue
<LoadingSkeleton v-if="isLoading" type="card" :rows="6" />
<ErrorState v-else-if="isError" :error="error" @retry="refetch()" />
<EmptyState v-else-if="!data?.items.length" ... />
<div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
  <KnowledgeBaseCard
    v-for="knowledgeBase in data.items"
    :key="knowledgeBase.id"
    :knowledge-base="knowledgeBase"
  />
</div>
```

Apply the same state order to documents, conversations, runs, Memory, MCP, models, and profile pages. An API failure must never be rendered as an empty list. Each empty state contains one icon, title, explanatory sentence, and primary action where the user has write permission.

- [ ] **Step 4: Preserve the first question when creating a conversation**

Refactor the question mutation so the conversation identifier is passed with the mutation variables instead of captured from the current route:

```ts
export interface SubmitQuestionVariables {
  conversation_id: string
  question: QuestionInput
}

export function useSubmitQuestion() {
  return useMutation({
    mutationFn: ({ conversation_id, question }: SubmitQuestionVariables) =>
      submitQuestion(conversation_id, question),
  })
}
```

In `ChatPage.handleSubmit`, calculate `targetConversationId`: use the current ID or await conversation creation, update the route, then submit the original query exactly once. Only append optimistic user/assistant messages after the target conversation exists. Disable submission until the create-and-submit sequence finishes.

- [ ] **Step 5: Implement AgentRun cancel, retry, and disconnect reconciliation**

Define exact transport types and functions:

```ts
export interface CancelAgentRunResult {
  run_id: string
  status: 'cancelled'
}

export interface RetryAgentRunResult {
  new_run_id: string
  retry_of_run_id: string
  status: 'queued'
}

export function cancelAgentRun(runId: string): Promise<CancelAgentRunResult>
export function retryAgentRun(runId: string): Promise<RetryAgentRunResult>
export function agentRunEventsUrl(runId: string): string
```

Both mutations use the shared API client and `VITE_API_BASE_URL`; remove the direct `fetch('/api/v1/...')` call from `ChatPage`.

Create `useAgentRunStream.ts` as the sole owner of the current `AbortController`. It must:

1. subscribe with Bearer authentication;
2. pass `after_sequence=last_sequence` only when `VITE_SSE_RESUME_ENABLED=true`;
3. abort on terminal events, logout, component disposal, and conversation route change;
4. on a non-terminal disconnect, call `getAgentRun(run_id)`;
5. refresh messages and AgentRun queries when the persisted run is terminal;
6. expose a reconnect action when the persisted run is still `queued` or `running`;
7. keep sequence deduplication in `agentRuntimeStore`.

Cancel must await `cancelAgentRun` successfully before aborting the local stream. Retry is shown only for a failed run, resets the runtime with `new_run_id`, derives `/api/v1/agent-runs/{new_run_id}/events`, and subscribes to the replacement run.

- [ ] **Step 6: Complete the document-side AI experience**

Remove the Task 9 placeholder from `DocumentAiPanel`. Reuse `ChatComposer`, `MessageList`, `AgentRunPanel`, `useSubmitQuestion`, and the stream composable rather than creating a second runtime implementation.

On the first submitted question, create one conversation in the current knowledge base, retain its ID for subsequent panel questions, and submit:

```ts
buildQuestionPayload(query, props.documentId)
```

When `VITE_DOCUMENT_SCOPE_ENABLED=false`, omit `document_id` and visibly label the panel `基于当前知识库`. When enabled, send `document_id` and label it `基于当前文档：{documentTitle}`. The “打开完整聊天” action opens the same conversation, not a new blank chat.

- [ ] **Step 7: Make knowledge and network citations navigable**

For a knowledge citation, route to:

```ts
router.push({
  name: 'document-detail',
  params: {
    kbId: citation.knowledge_base_id,
    documentId: citation.document_id,
  },
  query: {
    section: citation.source_location.section,
    quote: citation.quoted_text,
    fromConversation: currentConversationId,
  },
})
```

Network citations remain external anchors with `target="_blank"` and `rel="noopener noreferrer"`. In `DocumentWorkspacePage`, render sanitized document HTML through `CitationHighlight`. Locate the section using `CSS.escape`, wrap the matched quote in `<mark data-citation-highlight>`, scroll it into view, and remove the previous mark before applying a new one. If the quote is not found or `document_updated_at` differs from the current document, show `文档可能已更新，已定位到最接近章节` instead of silently approximating success.

- [ ] **Step 8: Repair production proxy and reproducible package metadata**

Use the Compose service name in all Nginx upstreams:

```nginx
proxy_pass http://api:8080;
```

Keep `proxy_buffering off`, `proxy_cache off`, HTTP/1.1, and the extended read timeout on the SSE location. Commit `web/.gitignore` and `web/pnpm-workspace.yaml`. Copy the workspace file before frozen installation:

```dockerfile
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
```

Document that the production container resolves `api` through the Compose network.

- [ ] **Step 9: Run static, route, visual, and runtime verification**

Run from `web/`:

```bash
pnpm install --frozen-lockfile
pnpm typecheck
pnpm lint
pnpm build
```

Run from the repository root where Docker is available:

```bash
docker compose --env-file .env -f deploy/docker-compose.yml config
docker compose --env-file .env -f deploy/docker-compose.yml up --build web
```

At 1280×720, 1440×900, and 1920×1080, capture and review `/knowledge-bases`, one chat, one document, `/runs`, `/memories`, `/mcp`, and `/settings/models`. Each screenshot must show readable navigation icons, a visible page header, and a loading, empty, error, or ready content state. Reject any screen containing only the navigation rail and a blank canvas.

Manually verify: first-question preservation, chat route shell, normal SSE completion, disconnect reconciliation, reconnect, cancel acknowledgement, retry, document-side question fallback, enabled document scope, knowledge citation navigation/highlight, stale citation warning, and network citation external-link safety.

- [ ] **Step 10: Update evidence and commit Task 15**

Record static checks separately from Docker/runtime and visual acceptance in `docs/FRONTEND.md`. Include viewport, route, state, result, and `request_id` for every failure. Do not mark unavailable backend contracts as passed.

```bash
git add web/src web/nginx.conf web/Dockerfile web/.gitignore web/pnpm-workspace.yaml web/README.md docs/FRONTEND.md docs/superpowers/specs/2026-08-01-memora-frontend-design.md docs/superpowers/plans/2026-08-01-memora-frontend-implementation.md
git commit -m "fix(web): complete frontend usability and runtime recovery"
```

---

## Backend Contract Gates

Claude must not silently invent backend behavior. Keep feature flags disabled until these contracts are implemented and verified:

1. `POST /conversations/{id}/questions` accepts optional `document_id` and validates ownership;
2. `GET /agent-runs/{run_id}/events` accepts `after_sequence` or standard `Last-Event-ID`;
3. the SSE endpoint defines whether first connection and reconnection replay history;
4. a safe file-read or short-lived signed-download endpoint exists before original PDF/DOCX preview is added;
5. the backend defines the normalized `document.content` format for all supported source types;
6. Nginx and every upstream proxy disable buffering for the SSE route.

Default frontend behavior before those gates are met:

```env
VITE_DOCUMENT_SCOPE_ENABLED=false
VITE_SSE_RESUME_ENABLED=false
```

## Final Completion Checklist

- [ ] All 15 task commits exist in order or have equivalent scoped commits.
- [ ] `pnpm install --frozen-lockfile`, typecheck, lint, and build exit 0.
- [ ] The Compose configuration renders successfully.
- [ ] No committed test files violate repository policy.
- [ ] No raw secret, password, token, or hidden reasoning is rendered or logged.
- [ ] No execution-mode selector exists.
- [ ] Documents remain read-only.
- [ ] Unsupported widths below 1280px show the desktop notice.
- [ ] SSE terminates cleanly on completion, failure, cancel, logout, and route leave.
- [ ] Knowledge and network citations are visually distinct and navigable.
- [ ] P0 manual chain results are recorded without conflating static checks and runtime acceptance.
- [ ] Every protected route renders inside AppShell through `RouterView`; no route shows only navigation and a blank canvas.
- [ ] Navigation, typography, surfaces, icons, focus states, and status colors match design section 12 exactly.
- [ ] The first question survives conversation creation and is submitted exactly once.
- [ ] AgentRun disconnect reconciliation, reconnect, acknowledged cancel, and failed-run retry are manually verified.
- [ ] Document AI submits through the shared conversation/runtime path and clearly labels knowledge-base versus document scope.
