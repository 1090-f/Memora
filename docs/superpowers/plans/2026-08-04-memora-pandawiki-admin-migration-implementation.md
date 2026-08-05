# Memora PandaWiki Admin Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the removed Vue frontend with a Memora-branded React/Vite admin application derived from PandaWiki, while connecting the currently available Memora auth and user APIs and establishing honest, typed boundaries for later business APIs.

**Architecture:** Keep a reduced pnpm workspace containing `admin`, `packages/icons`, `packages/themes`, and `packages/ui`; do not migrate the Next.js app. Route pages call feature hooks, feature hooks use TanStack Query, and transport is isolated in a Memora envelope-aware API client and authenticated SSE client. Redux stores only client-owned session/context/layout state.

**Tech Stack:** pnpm 10.12.1, React 19, TypeScript, Vite, React Router 7, MUI 7, `@ctzhian/ui`, Redux Toolkit, TanStack Query, Axios, Vitest, React Testing Library, MSW, Nginx.

## Global Constraints

- Use `PandaWiki-main/PandaWiki-main/web/admin` and its three workspace packages as the source; never modify the source directory during migration.
- Do not copy `PandaWiki-main/PandaWiki-main/web/app`, PandaWiki backend code, SDK code, or public-portal deployment files.
- Runtime APIs use only Memora `/api/v1`; do not add a PandaWiki response or route compatibility layer.
- Store the access token in `sessionStorage`, clear it on 401, and return to the originally requested route after login.
- Keep Memora DTO properties in `snake_case`.
- Do not show fake success data for unavailable backend domains; show an explicit unavailable state and disable mutations.
- Do not reintroduce Vue, online document editing, full mobile layouts, or the Next.js public portal.
- Do not delete `PandaWiki-main/`; removal is a separate user decision after acceptance.
- Preserve unrelated changes in the existing dirty worktree and stage only files belonging to the current task.

---

## File Structure

### Workspace and shared packages

- `web/package.json`: workspace commands and shared dependency versions.
- `web/pnpm-workspace.yaml`: includes only `admin` and `packages/*`.
- `web/packages/icons/`: reusable icons, published in-workspace as `@memora/icons`.
- `web/packages/themes/`: reusable MUI tokens, published in-workspace as `@memora/themes`.
- `web/packages/ui/`: reusable primitives, published in-workspace as `@memora/ui`.

### Admin application

- `web/admin/src/app/`: providers, route definitions, route guards, capability registry.
- `web/admin/src/api/`: envelope client, error model, SSE transport, query keys.
- `web/admin/src/features/`: isolated auth, user, knowledge-base, document, conversation, agent-run, memory, MCP, search, and model domains.
- `web/admin/src/layouts/`: authenticated shell, anonymous shell, chat workspace, document workspace.
- `web/admin/src/components/shared/`: error, empty, loading, unavailable, notification, and request-ID components.
- `web/admin/src/store/`: Redux slices only for auth, workspace context, and layout preferences.
- `web/admin/src/styles/`: Memora tokens and retained PandaWiki visual primitives.

### Delivery

- `web/Dockerfile`: build the admin workspace and serve `admin/dist`.
- `web/nginx.conf`: SPA fallback, `/api/v1` proxy, health proxy, and unbuffered Agent SSE.
- `docs/FRONTEND.md`: React architecture, commands, contracts, and rollout status.

---

### Task 1: Import the reduced pnpm workspace

**Files:**
- Create from reference: `web/admin/**`
- Create from reference: `web/packages/icons/**`
- Create from reference: `web/packages/themes/**`
- Create from reference: `web/packages/ui/**`
- Create from reference: `web/package.json`
- Create from reference: `web/pnpm-lock.yaml`
- Create from reference: `web/pnpm-workspace.yaml`
- Create from reference: `web/prettier.config.js`
- Create from reference: `web/tsconfig.base.json`
- Modify: `web/pnpm-workspace.yaml`
- Modify: `web/package.json`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: read-only source tree `PandaWiki-main/PandaWiki-main/web`.
- Produces: a pnpm workspace where `pnpm --filter memora-admin build` targets the only application.

- [ ] **Step 1: Record the import boundary before copying**

Run from the repository root:

```powershell
git status --short
Get-ChildItem PandaWiki-main\PandaWiki-main\web\admin
Get-ChildItem PandaWiki-main\PandaWiki-main\web\packages
```

Expected: the historical `web/` appears deleted, and the reference contains `admin`, `icons`, `themes`, and `ui`.

- [ ] **Step 2: Restore only the selected workspace content**

Use PowerShell `Copy-Item` to copy the root workspace files, `admin`, and `packages`; do not copy `app`:

```powershell
New-Item -ItemType Directory -Path web -Force
Copy-Item -Recurse PandaWiki-main\PandaWiki-main\web\admin web\admin
Copy-Item -Recurse PandaWiki-main\PandaWiki-main\web\packages web\packages
Copy-Item PandaWiki-main\PandaWiki-main\web\package.json,PandaWiki-main\PandaWiki-main\web\pnpm-lock.yaml,PandaWiki-main\PandaWiki-main\web\pnpm-workspace.yaml,PandaWiki-main\PandaWiki-main\web\prettier.config.js,PandaWiki-main\PandaWiki-main\web\tsconfig.base.json web
```

Expected: `web/app` does not exist.

- [ ] **Step 3: Reduce workspace membership and application scripts**

Set `web/pnpm-workspace.yaml` to:

```yaml
packages:
  - 'admin'
  - 'packages/*'
```

Change the root package name to `@memora/web`, keep pnpm `10.12.1`, and expose:

```json
{
  "scripts": {
    "build": "pnpm --filter memora-admin build",
    "dev": "pnpm --filter memora-admin dev",
    "lint": "pnpm --filter memora-admin lint",
    "typecheck": "pnpm --filter memora-admin typecheck",
    "test": "pnpm --filter memora-admin test"
  }
}
```

- [ ] **Step 4: Prevent generated output from entering Git**

Ensure `.gitignore` contains:

```gitignore
web/node_modules/
web/**/node_modules/
web/**/dist/
web/**/coverage/
web/admin/.env.local
```

- [ ] **Step 5: Install from the imported lockfile**

Run:

```powershell
Set-Location web
corepack enable
pnpm install --frozen-lockfile
```

Expected: installation succeeds without resolving `web/app`.

- [ ] **Step 6: Capture the expected pre-migration build failure**

Run:

```powershell
pnpm --filter memora-admin build
```

Expected: failure because the imported package is still named `panda-wiki-admin` and/or contains PandaWiki-only generated/API dependencies. Record the exact failure in the task notes; do not weaken TypeScript to bypass it.

- [ ] **Step 7: Commit the isolated import**

```powershell
git add .gitignore web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml web/prettier.config.js web/tsconfig.base.json web/admin web/packages
git commit -m "build(web): import PandaWiki admin foundation"
```

### Task 2: Rebrand packages and establish a clean build/test baseline

**Files:**
- Modify: `web/admin/package.json`
- Modify: `web/admin/index.html`
- Modify: `web/admin/vite.config.ts`
- Modify: `web/admin/src/main.tsx`
- Modify: `web/admin/src/App.tsx`
- Modify: `web/packages/icons/package.json`
- Modify: `web/packages/themes/package.json`
- Modify: `web/packages/ui/package.json`
- Modify: all imported `web/**/*.ts` and `web/**/*.tsx` references from `@panda-wiki/*` to `@memora/*`
- Create: `web/admin/vitest.config.ts`
- Create: `web/admin/src/test/setup.ts`
- Create: `web/admin/src/test/smoke.test.tsx`

**Interfaces:**
- Consumes: reduced workspace from Task 1.
- Produces: `memora-admin`, `@memora/icons`, `@memora/themes`, and `@memora/ui`; commands `lint`, `typecheck`, `test`, and `build`.

- [ ] **Step 1: Write a failing brand smoke test**

Create `web/admin/src/test/smoke.test.tsx`:

```tsx
import { describe, expect, it } from 'vitest';

describe('Memora frontend baseline', () => {
  it('uses the Memora product name', () => {
    expect(document.title).toBe('Memora');
  });
});
```

- [ ] **Step 2: Add test dependencies and scripts**

Add `@tanstack/react-query` to admin dependencies and add `vitest`, `jsdom`, `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`, and `msw` to dev dependencies. Define:

```json
{
  "scripts": {
    "lint": "eslint src --max-warnings=0",
    "typecheck": "tsc -b --pretty false",
    "test": "vitest run",
    "test:watch": "vitest",
    "build": "pnpm typecheck && vite build"
  }
}
```

- [ ] **Step 3: Configure Vitest and run the failing test**

Create `web/admin/vitest.config.ts` with `jsdom`, the `@` alias, and `src/test/setup.ts`; import `@testing-library/jest-dom/vitest` in setup.

Run:

```powershell
Set-Location web\admin
pnpm test -- src/test/smoke.test.tsx
```

Expected: FAIL because the imported HTML title is not `Memora`.

- [ ] **Step 4: Rename workspace packages and all imports**

Apply these exact names:

```text
panda-wiki-admin  → memora-admin
@panda-wiki/icons → @memora/icons
@panda-wiki/themes → @memora/themes
@panda-wiki/ui → @memora/ui
```

Update the lockfile using `pnpm install --lockfile-only`; do not hand-edit lockfile integrity data.

- [ ] **Step 5: Remove PandaWiki-only boot behavior**

In `src/main.tsx`, remove `panda-wiki.css`, basename mutation, and Panda-specific globals. In `src/App.tsx`, remove license requests, generated Panda request imports, and `localStorage` token checks. Set `<title>Memora</title>` and Memora metadata in `index.html`.

- [ ] **Step 6: Simplify Vite configuration**

Remove generated route metadata and `/static-file`/`/share` proxies. Keep the `@` alias and use:

```ts
server: {
  host: '0.0.0.0',
  proxy: {
    '/api/v1': {
      target: env.VITE_API_PROXY_TARGET || 'http://localhost:8080',
      changeOrigin: true,
    },
  },
}
```

Set build assets to `assets` and preserve useful manual chunks only when every referenced dependency remains installed.

- [ ] **Step 7: Remove unreachable PandaWiki feature code**

Delete imported admin modules dedicated solely to contribution, release, feedback, statistics, public-site decoration, licensing, collaborative editing, publication, user groups, and generated Panda request clients. Before each deletion, search imports and remove callers in the same change.

- [ ] **Step 8: Verify the clean baseline**

Run:

```powershell
Set-Location web
pnpm test
pnpm lint
pnpm typecheck
pnpm build
Get-ChildItem -Path . -Recurse -File -Include *.ts,*.tsx,*.json,*.html | Select-String -Pattern 'PandaWiki|panda-wiki|@panda-wiki'
```

Expected: all commands pass and the brand scan returns no runtime/product references; retained license notices are allowed only when required by the source license.

- [ ] **Step 9: Commit the baseline**

```powershell
git add web
git commit -m "refactor(web): establish Memora React baseline"
```

### Task 3: Build the application shell, routing, and capability states

**Files:**
- Create: `web/admin/src/app/providers.tsx`
- Create: `web/admin/src/app/router.tsx`
- Create: `web/admin/src/app/RequireAuth.tsx`
- Create: `web/admin/src/app/capabilities.ts`
- Create: `web/admin/src/layouts/AppShell.tsx`
- Create: `web/admin/src/layouts/AnonymousLayout.tsx`
- Create: `web/admin/src/components/shared/LoadingState.tsx`
- Create: `web/admin/src/components/shared/EmptyState.tsx`
- Create: `web/admin/src/components/shared/ErrorState.tsx`
- Create: `web/admin/src/components/shared/UnavailableState.tsx`
- Create: `web/admin/src/components/shared/RequestIdCopy.tsx`
- Create: `web/admin/src/features/*/pages/*.tsx` for every route listed below
- Create: `web/admin/src/app/router.test.tsx`
- Modify: `web/admin/src/main.tsx`
- Modify: `web/admin/src/App.tsx`

**Interfaces:**
- Produces: `CapabilityKey`, `CapabilityStatus`, `RequireAuth`, and the canonical route table.
- Route table: `/login`, `/knowledge-bases`, `/kb/:kbId/docs/:documentId?`, `/chat/:kbId/:conversationId?`, `/runs/:runId?`, `/memories`, `/mcp`, `/kb/:kbId/search-test`, `/kb/:kbId/settings`, `/settings/profile`, `/settings/models`.

- [ ] **Step 1: Write failing route tests**

Test that `/login` renders without an authenticated session, a protected URL redirects to `/login?redirect=...`, and each protected route renders its page title when authenticated.

```tsx
it('preserves a protected destination during login redirect', async () => {
  renderRouter('/memories', { authenticated: false });
  expect(await screen.findByRole('heading', { name: '登录 Memora' })).toBeInTheDocument();
  expect(window.location.search).toContain('redirect=%2Fmemories');
});
```

- [ ] **Step 2: Run the route test to verify failure**

```powershell
Set-Location web\admin
pnpm test -- src/app/router.test.tsx
```

Expected: FAIL because the canonical router and guard do not exist.

- [ ] **Step 3: Define capability status explicitly**

Create:

```ts
export type CapabilityKey =
  | 'auth'
  | 'user'
  | 'knowledgeBase'
  | 'document'
  | 'conversation'
  | 'agentRun'
  | 'memory'
  | 'mcp'
  | 'search'
  | 'model';

export type CapabilityStatus = 'available' | 'backend_pending';

export const capabilities: Record<CapabilityKey, CapabilityStatus> = {
  auth: 'available',
  user: 'available',
  knowledgeBase: 'backend_pending',
  document: 'backend_pending',
  conversation: 'backend_pending',
  agentRun: 'backend_pending',
  memory: 'backend_pending',
  mcp: 'backend_pending',
  search: 'backend_pending',
  model: 'backend_pending',
};
```

- [ ] **Step 4: Implement providers and the route guard**

Compose MUI/`@ctzhian/ui`, Redux, QueryClient, BrowserRouter, and the global notification host in `providers.tsx`. `RequireAuth` must read the session store and render `<Navigate to='/login' state={{ redirect: location.pathname + location.search }} replace />` when unauthenticated.

- [ ] **Step 5: Implement the Memora shell**

Adapt PandaWiki Sidebar/Header visuals but replace navigation with the canonical Memora routes. Use semantic links, visible focus states, `aria-current='page'`, a 60px navigation rail, 64px page header, and an unsupported-width notice below 1280px.

- [ ] **Step 6: Implement shared route states**

`UnavailableState` accepts `{ title, description, capability }`, renders an informational status rather than an error, and never exposes an enabled mutation button. `ErrorState` accepts `AppError` and includes `RequestIdCopy` only when `request_id` exists.

- [ ] **Step 7: Add route pages with honest first-stage content**

Each unavailable route must render its final page shell and `UnavailableState`, not fake list items. `/login` and `/settings/profile` may render a loading skeleton until their real implementation in Task 5.

- [ ] **Step 8: Verify routing and commit**

```powershell
Set-Location web
pnpm test
pnpm lint
pnpm typecheck
pnpm build
git add admin/src
git commit -m "feat(web): add Memora application shell and routes"
```

### Task 4: Implement the Memora API client and authenticated SSE transport

**Files:**
- Create: `web/admin/src/api/types.ts`
- Create: `web/admin/src/api/errors.ts`
- Create: `web/admin/src/api/client.ts`
- Create: `web/admin/src/api/queryKeys.ts`
- Create: `web/admin/src/api/sse.ts`
- Create: `web/admin/src/api/client.test.ts`
- Create: `web/admin/src/api/sse.test.ts`
- Create: `web/admin/src/test/server.ts`
- Modify: `web/admin/src/test/setup.ts`

**Interfaces:**
- Produces: `ApiEnvelope<T>`, `AppError`, `apiRequest<T>()`, `setUnauthorizedHandler()`, `readSseStream()`.

- [ ] **Step 1: Define and test the envelope contract**

Use these types:

```ts
export interface ApiEnvelope<T> {
  code: string;
  message: string;
  data?: T;
  details?: unknown;
  request_id: string;
}

export class AppError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly httpStatus: number,
    public readonly details: unknown,
    public readonly requestId: string,
  ) {
    super(message);
  }
}
```

Tests must cover success unwrapping, non-OK envelopes, non-JSON 502 responses, network errors, Bearer injection, no token header when logged out, and one 401 callback.

- [ ] **Step 2: Run client tests to verify failure**

```powershell
Set-Location web\admin
pnpm test -- src/api/client.test.ts
```

Expected: FAIL because `apiRequest` is not defined.

- [ ] **Step 3: Implement `apiRequest<T>()`**

Use a single Axios instance with base URL `import.meta.env.VITE_API_BASE_URL || '/api/v1'`. Read the token per request from the auth session adapter, unwrap only `code === 'OK'`, and throw `AppError` otherwise. Notification UI belongs to feature/UI boundaries, not the interceptor.

- [ ] **Step 4: Define and test SSE event parsing**

Use:

```ts
export interface SseEvent<T = unknown> {
  id?: string;
  event: string;
  data: T;
  sequence?: number;
}

export interface SseOptions {
  signal: AbortSignal;
  afterSequence?: number;
  onEvent: (event: SseEvent) => void;
}
```

Tests must split UTF-8 and SSE frames across chunks, ignore duplicate sequences, parse terminal events, propagate protocol `error`, send `after_sequence`, and abort without reporting a transport failure.

- [ ] **Step 5: Implement authenticated fetch streaming**

`readSseStream(url, options)` must send `Accept: text/event-stream` and Bearer Token, parse `id:`, `event:`, and multi-line `data:`, close on `done`, `completed`, or `error`, and never use native `EventSource`.

- [ ] **Step 6: Verify transport and commit**

```powershell
Set-Location web
pnpm test
pnpm lint
pnpm typecheck
git add admin/src/api admin/src/test
git commit -m "feat(web): add Memora API and SSE transports"
```

### Task 5: Connect authentication and current-user features

**Files:**
- Create: `web/admin/src/features/auth/types.ts`
- Create: `web/admin/src/features/auth/api.ts`
- Create: `web/admin/src/features/auth/session.ts`
- Create: `web/admin/src/features/auth/authSlice.ts`
- Create: `web/admin/src/features/auth/pages/LoginPage.tsx`
- Create: `web/admin/src/features/auth/auth.test.tsx`
- Create: `web/admin/src/features/user/types.ts`
- Create: `web/admin/src/features/user/api.ts`
- Create: `web/admin/src/features/user/pages/ProfilePage.tsx`
- Create: `web/admin/src/features/user/user.test.tsx`
- Modify: `web/admin/src/store/index.ts`
- Modify: `web/admin/src/app/RequireAuth.tsx`
- Modify: `web/admin/src/layouts/AppShell.tsx`

**Interfaces:**
- Consumes: `apiRequest<T>()` and unauthorized callback from Task 4.
- Produces: `LoginRequest`, `LoginResponse`, `User`, session accessors, and Query hooks for `/users/me`.
- Full backend contracts: `POST /api/v1/auth/login`, `POST /api/v1/auth/logout`, `GET /api/v1/users/me`, `PATCH /api/v1/users/me`, and `PATCH /api/v1/users/me/password`.

- [ ] **Step 1: Add exact DTOs and failing auth tests**

```ts
export interface User {
  id: string;
  username: string;
  nickname: string;
  email: string;
  avatar_url: string | null;
  bio?: string | null;
}

export interface LoginRequest { account: string; password: string }
export interface LoginResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: User;
}
```

Test successful login persistence and redirect, rejected credentials with `request_id`, logout cleanup, startup session restore, and 401 cleanup.

- [ ] **Step 2: Run auth tests to verify failure**

```powershell
Set-Location web\admin
pnpm test -- src/features/auth/auth.test.tsx
```

Expected: FAIL because the session and login page are not implemented.

- [ ] **Step 3: Implement session storage and auth API**

Use one key, `memora.auth`, containing `{ access_token, expires_at, user }`. Reject malformed or expired JSON during restore. Implement:

```ts
login(input: LoginRequest): Promise<LoginResponse>
logout(): Promise<void>
```

Logout clears local state in `finally`, including when the network request fails.

- [ ] **Step 4: Implement accessible login flow**

Adapt the PandaWiki visual layout but use `account` and `password`, visible labels, native submit semantics, disabled/loading state, field errors, and a general `AppError` summary with copyable request ID. Never log credentials.

- [ ] **Step 5: Add exact user APIs and failing profile tests**

Implement type signatures:

```ts
getCurrentUser(): Promise<User>
updateCurrentUser(input: {
  nickname?: string;
  avatar_url?: string;
  bio?: string;
  email?: string;
}): Promise<User>
changePassword(input: {
  old_password: string;
  new_password: string;
}): Promise<{ password_changed: boolean }>
```

Tests must cover profile loading/update, email validation, password minimum length 12, success form reset, and server error rendering.

- [ ] **Step 6: Implement profile and password forms**

Use TanStack Query key `['users', 'me']`, invalidate/update it after profile mutation, and update the auth slice user after success. Clear password fields after both success and explicit form reset.

- [ ] **Step 7: Verify real backend contract compatibility**

Start the Go API using the documented local environment, then manually verify login, `GET /users/me`, profile update, password validation, logout, and 401 redirect. Record any unavailable local dependency as an environment blocker; do not replace this check with production mocks.

- [ ] **Step 8: Run automated gates and commit**

```powershell
Set-Location web
pnpm test
pnpm lint
pnpm typecheck
pnpm build
git add admin/src
git commit -m "feat(web): connect Memora authentication and profile"
```

### Task 6: Adapt knowledge-base and read-only document workspaces

**Files:**
- Create: `web/admin/src/features/knowledge-base/types.ts`
- Create: `web/admin/src/features/knowledge-base/api.ts`
- Create: `web/admin/src/features/knowledge-base/pages/KnowledgeBaseListPage.tsx`
- Create: `web/admin/src/features/knowledge-base/pages/KnowledgeBaseSettingsPage.tsx`
- Create: `web/admin/src/features/document/types.ts`
- Create: `web/admin/src/features/document/api.ts`
- Create: `web/admin/src/features/document/components/KnowledgeTree.tsx`
- Create: `web/admin/src/features/document/components/ImportDrawer.tsx`
- Create: `web/admin/src/features/document/components/DocumentViewer.tsx`
- Create: `web/admin/src/features/document/pages/DocumentWorkspacePage.tsx`
- Create: `web/admin/src/layouts/DocumentWorkspace.tsx`
- Create: `web/admin/src/features/document/document.test.tsx`
- Adapt from reference: `web/admin/src/pages/document/layout/**`
- Adapt from reference: `web/admin/src/components/KB/**`

**Interfaces:**
- Produces: typed domain shapes matching the Memora API specification and read-only page components.
- Capability behavior: `backend_pending` prevents network calls and mutations; `available` enables Query hooks without changing page components.

- [ ] **Step 1: Write tests for pending and available capability states**

Test that pending knowledge-base/document capabilities make zero HTTP calls, show the final shell, disable create/import actions, and explain the missing backend route. Test the available state with MSW responses matching Memora envelopes.

- [ ] **Step 2: Run document tests to verify failure**

```powershell
Set-Location web\admin
pnpm test -- src/features/document/document.test.tsx
```

Expected: FAIL because the feature components and capability gating do not exist.

- [ ] **Step 3: Define domain types from the Memora API document**

Define separate `KnowledgeBase`, `DirectoryNode`, `Document`, `ImportTask`, and pagination types. Keep IDs as strings and statuses as closed unions from the API document. Do not reuse PandaWiki generated request types.

- [ ] **Step 4: Adapt the knowledge-base UI**

Reuse PandaWiki card, selection, dialog, and form primitives, but remove publishing, edition/license, public site, model wizard, and organization roles. The list page owns only composition; queries and mutations live in feature hooks.

- [ ] **Step 5: Adapt the document workspace as read-only**

Use a resizable tree/sidebar, breadcrumb/header, sanitized `DocumentViewer`, status panel, and import drawer. Do not copy full-text editor, Yjs collaboration, version publishing, or rollback UI.

- [ ] **Step 6: Implement API modules behind capability checks**

Export typed functions for the documented knowledge-base, directory, document, and import endpoints, but only invoke them when the capability registry says `available`. Keep endpoint strings in the feature API modules so backend activation is a one-line capability change plus contract verification.

- [ ] **Step 7: Verify and commit**

```powershell
Set-Location web
pnpm test
pnpm lint
pnpm typecheck
pnpm build
git add admin/src/features/knowledge-base admin/src/features/document admin/src/layouts
git commit -m "feat(web): add knowledge and document workspaces"
```

### Task 7: Build the three-column chat workspace and Agent event model

**Files:**
- Create: `web/admin/src/features/conversation/types.ts`
- Create: `web/admin/src/features/conversation/api.ts`
- Create: `web/admin/src/features/conversation/events.ts`
- Create: `web/admin/src/features/conversation/components/ConversationSidebar.tsx`
- Create: `web/admin/src/features/conversation/components/MessageList.tsx`
- Create: `web/admin/src/features/conversation/components/ChatComposer.tsx`
- Create: `web/admin/src/features/conversation/pages/ChatPage.tsx`
- Create: `web/admin/src/features/agent-run/types.ts`
- Create: `web/admin/src/features/agent-run/eventReducer.ts`
- Create: `web/admin/src/features/agent-run/components/AgentRunPanel.tsx`
- Create: `web/admin/src/features/agent-run/pages/AgentRunListPage.tsx`
- Create: `web/admin/src/features/agent-run/pages/AgentRunDetailPage.tsx`
- Create: `web/admin/src/layouts/ChatWorkspace.tsx`
- Create: `web/admin/src/store/layoutSlice.ts`
- Create: `web/admin/src/features/conversation/chat.test.tsx`
- Create: `web/admin/src/features/agent-run/eventReducer.test.ts`
- Adapt from reference: `web/admin/src/pages/conversation/**`

**Interfaces:**
- Consumes: `readSseStream()`, QueryClient, workspace context.
- Produces: deterministic `reduceAgentEvent(state, event)` and a chat UI independent of transport details.

- [ ] **Step 1: Write reducer tests from the documented event sequence**

Cover router selection, answer deltas, plan steps, ReAct rounds, tool calls, citations, usage, completed, cancelled, error, duplicate sequence, and out-of-order replay. Assert that complete model reasoning is never stored.

- [ ] **Step 2: Run reducer tests to verify failure**

```powershell
Set-Location web\admin
pnpm test -- src/features/agent-run/eventReducer.test.ts
```

Expected: FAIL because `reduceAgentEvent` is missing.

- [ ] **Step 3: Implement a pure Agent event reducer**

The reducer must preserve answer text after failure/cancellation, retain the highest processed sequence, expose resumable status, and keep only user-visible router/plan/tool summaries.

- [ ] **Step 4: Write chat interaction tests**

Test conversation search, create deduplication, selected state, suggestion-to-draft without auto-submit, send, stop, reconnect, citation activation, long tool/error wrapping, right-panel resize clamped to 320–480px, and layout persistence.

- [ ] **Step 5: Build the three-column workspace**

Use approximately 280px for conversations, the remaining width for messages/composer, and 360px for the Agent panel. Keep the composer visible while streaming. Collapse and resized width use `memora.layout.chat` in local storage because they are non-sensitive UI preferences.

- [ ] **Step 6: Connect capability-gated conversation APIs**

Define documented functions for conversation CRUD, question submission, cancellation, retry, run detail, citation detail, and event streaming. When capability is pending, render the complete empty workspace with a disabled composer and explanatory status; do not open a stream.

- [ ] **Step 7: Verify and commit**

```powershell
Set-Location web
pnpm test
pnpm lint
pnpm typecheck
pnpm build
git add admin/src/features/conversation admin/src/features/agent-run admin/src/layouts/ChatWorkspace.tsx admin/src/store/layoutSlice.ts
git commit -m "feat(web): add chat and Agent run workspaces"
```

### Task 8: Add management pages, delivery configuration, and full acceptance gates

**Files:**
- Create: `web/admin/src/features/memory/types.ts`
- Create: `web/admin/src/features/memory/api.ts`
- Create: `web/admin/src/features/memory/pages/MemoryPage.tsx`
- Create: `web/admin/src/features/mcp/types.ts`
- Create: `web/admin/src/features/mcp/api.ts`
- Create: `web/admin/src/features/mcp/pages/McpPage.tsx`
- Create: `web/admin/src/features/search/types.ts`
- Create: `web/admin/src/features/search/api.ts`
- Create: `web/admin/src/features/search/pages/SearchTestPage.tsx`
- Create: `web/admin/src/features/model/types.ts`
- Create: `web/admin/src/features/model/api.ts`
- Create: `web/admin/src/features/model/pages/ModelSettingsPage.tsx`
- Create: `web/admin/src/features/management/management.test.tsx`
- Create: `web/.env.example`
- Create: `web/Dockerfile`
- Create: `web/nginx.conf`
- Modify: `docs/FRONTEND.md`
- Modify: `README.md`

**Interfaces:**
- Produces: remaining canonical routes, production admin image, and documented frontend operation.

- [ ] **Step 1: Write management capability tests**

For Memory, MCP, search, and model pages, assert that pending capabilities make no HTTP requests, mutation controls are disabled, page titles and explanations are visible, and `request_id` is shown for simulated available-mode errors.

- [ ] **Step 2: Run management tests to verify failure**

```powershell
Set-Location web\admin
pnpm test -- src/features/management/management.test.tsx
```

Expected: FAIL until all management routes use the shared capability/error pattern.

- [ ] **Step 3: Implement typed management pages**

Reuse PandaWiki form/card/table primitives where they match Memora, but remove write-capable MCP tools, Panda licensing, bots, public site customization, and organizational roles. Keep MCP tools read-only and make network capability status explicit.

- [ ] **Step 4: Add production environment configuration**

Create `web/.env.example`:

```dotenv
VITE_API_BASE_URL=/api/v1
VITE_API_PROXY_TARGET=http://localhost:8080
VITE_SSE_RESUME_ENABLED=false
VITE_DOCUMENT_SCOPE_ENABLED=false
```

- [ ] **Step 5: Add the admin Docker image**

Build from `node:24-alpine`, enable Corepack, install the frozen workspace lockfile, run `pnpm --filter memora-admin build`, and copy `web/admin/dist` into an unprivileged-compatible Nginx runtime image.

- [ ] **Step 6: Configure Nginx safely**

Provide SPA fallback, immutable caching only for hashed `/assets/`, no-cache for `index.html`, proxy `/api/v1` and health routes to `memora-server:8080`, and use this dedicated SSE location before the general API location:

```nginx
location ~ ^/api/v1/agent-runs/[^/]+/events$ {
    proxy_pass http://memora-server:8080;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 1h;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Connection '';
}
```

- [ ] **Step 7: Rewrite frontend documentation**

Document the React workspace, exact commands, route map, current capability table, Memora envelope, session behavior, SSE behavior, development proxy, production proxy, and the backend activation checklist. Remove Vue-specific instructions.

- [ ] **Step 8: Run the complete automated gate**

```powershell
Set-Location web
pnpm install --frozen-lockfile
pnpm test
pnpm lint
pnpm typecheck
pnpm build
```

Expected: all commands exit 0 and `web/admin/dist/index.html` exists.

- [ ] **Step 9: Run repository and security scans**

```powershell
Get-ChildItem -Path web -Recurse -File -Include *.ts,*.tsx,*.json,*.html,*.css,*.md | Select-String -Pattern 'PandaWiki|panda_wiki_token|@panda-wiki|v-html|EventSource'
git status --short
```

Expected: no runtime PandaWiki brand/token/package references, no unsafe Vue HTML directive, no native EventSource, and no unrelated files staged.

- [ ] **Step 10: Perform desktop visual and accessibility acceptance**

At 1280, 1440, and 1920px widths, verify every canonical route has a title and loading/empty/error/ready-or-unavailable state; no critical action is obscured; keyboard focus reaches navigation, forms, dialogs, chat controls, and panel collapse; state is not represented by color alone; long IDs and errors wrap without horizontal overflow.

- [ ] **Step 11: Perform the available backend smoke path**

Against the local Go service, verify login → current user → profile update → password validation → logout → protected-route redirect. Verify pending routes do not generate 404 noise in the browser network panel.

- [ ] **Step 12: Commit delivery and documentation**

```powershell
git add web docs/FRONTEND.md README.md
git commit -m "build(web): complete Memora admin delivery"
```

---

## Backend Capability Activation Checklist

When a Memora backend domain becomes available, use this fixed sequence rather than redesigning the page:

- [ ] Confirm the implemented route and DTO against the current API document and Go handler.
- [ ] Add MSW success, validation, unauthorized, not-found, conflict, limit, and upstream-failure fixtures.
- [ ] Run the feature test in `backend_pending` mode and confirm zero requests.
- [ ] Switch the single capability entry to `available` in the test.
- [ ] Run the feature test and confirm the expected Memora envelope and DTO are consumed.
- [ ] Switch the production capability only after the local Go route exists.
- [ ] Manually exercise the domain flow and record request IDs for failures.
- [ ] Run `pnpm test`, `pnpm lint`, `pnpm typecheck`, and `pnpm build` before committing.

## Final Acceptance Criteria

- The active frontend is `web/admin`; no Next.js app or Vue runtime is present.
- `web/` installs reproducibly with pnpm 10.12.1 and the frozen lockfile.
- Login, logout, current user, profile update, and password change use the real current Go APIs.
- Every documented Memora route is navigable and renders a complete state.
- Pending backend domains make no production requests and do not display fabricated success data.
- API and SSE transports use Memora authentication, envelope, error, request-ID, cancellation, and resume rules.
- PandaWiki-specific brands, routes, generated clients, licensing, publishing, public portal, and collaboration code are absent from the runtime bundle.
- Automated gates pass, production assets build, and desktop visual/accessibility acceptance is recorded.
