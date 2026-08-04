# Memora Immersive Frontend Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有 Memora Vue 3 前端升级为已确认的 B2 平衡沉浸视觉，并落实 L2 专注问答与 D2 阅读优先布局。

**Architecture:** 保留现有 Feature、Vue Query、Pinia 和 API 边界，只在 Design Token、全局壳层、工作区壳层与展示组件中建立新的视觉系统。业务组件继续负责数据和事件；布局组件只负责表面、尺寸、折叠和响应式行为。

**Tech Stack:** Vue 3、TypeScript、Vite、Tailwind CSS 4、Pinia、Lucide Vue Next、Vitest、Vue Test Utils、jsdom

## Global Constraints

- 不修改后端 API、数据库、Agent 业务语义和现有路由地址。
- 保留工作区中的现有未提交修改，不覆盖与本任务无关的文件。
- 视觉基准为 B2；聊天布局为 L2；文档布局为 D2。
- 低于 1280 像素时折叠非核心面板，不使用阻断式“不支持”页面。
- 动效必须支持 `prefers-reduced-motion`。
- 本轮不实现完整移动端、暗色主题、富文本编辑器或新的 Agent 工作流。

---

### Task 1: Test Harness and Visual Foundation

**Files:**
- Modify: `web/package.json`
- Modify: `web/pnpm-lock.yaml`
- Modify: `web/vite.config.ts`
- Modify: `web/src/styles/tokens.css`
- Modify: `web/src/styles/index.css`

**Interfaces:**
- Produces: CSS variables `--memora-ambient-violet`, `--memora-ambient-blue`, `--memora-glass`, `--memora-glass-strong`, `--memora-text-soft`, `--memora-shadow-sm`, `--memora-shadow-md`, `--memora-inspector-rail-width`.
- Produces: reusable CSS classes `.memora-ambient`, `.memora-glass`, `.memora-surface`, `.memora-page`, `.memora-page-header`, `.memora-focus-ring`.

- [ ] **Step 1: Add the test runner**

Run: `pnpm add -D vitest @vue/test-utils jsdom`

Expected: `package.json` and `pnpm-lock.yaml` include all three development dependencies.

- [ ] **Step 2: Configure Vitest and add scripts**

Add `"test": "vitest run"` to `web/package.json` and add this block to `web/vite.config.ts`:

```ts
test: {
  environment: 'jsdom',
  globals: true,
},
```

- [ ] **Step 3: Implement the B2 visual foundation**

Expand `tokens.css` with the interfaces listed above. Update `index.css` with ambient pseudo-elements, glass fallback surfaces, readable surfaces, page containers, selection color, custom scrollbars, focus treatment and reduced-motion rules.

- [ ] **Step 4: Verify the CSS configuration through the consumer build**

Run: `pnpm typecheck`

Expected: exit code 0. CSS configuration is declarative and receives no source-text test; observable layout behavior is tested in Tasks 2–6.

### Task 2: Global App Shell

**Files:**
- Create: `web/src/layouts/app-shell-navigation.ts`
- Create: `web/src/layouts/app-shell-navigation.test.ts`
- Modify: `web/src/layouts/AppShell.vue`
- Modify: `web/src/layouts/UnsupportedViewport.vue`

**Interfaces:**
- Produces: `APP_NAV_ITEMS` and `getActiveNavName(path: string): string | null`.
- Consumes: B2 surface classes from Task 1 and existing auth/workspace stores.

- [ ] **Step 1: Write the failing navigation test**

Test exact route mappings for chat, document, runs, memories, MCP, model settings and profile settings, including `/knowledge-bases` mapping to documents.

- [ ] **Step 2: Run the test and verify RED**

Run: `pnpm test -- src/layouts/app-shell-navigation.test.ts`

Expected: FAIL because `app-shell-navigation.ts` does not exist.

- [ ] **Step 3: Implement navigation mapping**

Create typed `APP_NAV_ITEMS` and `getActiveNavName`; update `AppShell.vue` to consume them, render ambient layers, use the glass navigation rail, improve the knowledge-base switcher, add active markers and preserve keyboard/search/user behavior.

- [ ] **Step 4: Replace the viewport blocker with graceful fallback**

Change `UnsupportedViewport.vue` from a full-page blocker into a compact warning banner. Keep the application mounted below 1280 pixels and let workspace shells collapse secondary panels.

- [ ] **Step 5: Run the test and verify GREEN**

Run: `pnpm test -- src/layouts/app-shell-navigation.test.ts`

Expected: all route mapping cases PASS.

### Task 3: L2 and D2 Workspace Shell Behavior

**Files:**
- Modify: `web/src/stores/layout.ts`
- Create: `web/src/stores/layout.test.ts`
- Modify: `web/src/layouts/ChatWorkspaceShell.vue`
- Modify: `web/src/layouts/DocumentWorkspaceShell.vue`

**Interfaces:**
- Produces: default `inspector_collapsed = true` for first-time users.
- Produces: `setInspectorCollapsed(collapsed: boolean): void`.
- Consumes: `--memora-inspector-rail-width`, `--memora-inspector-width`, `--memora-sidebar-width`.

- [ ] **Step 1: Write failing layout-store tests**

Test that a first-time store starts collapsed, stored values are restored, toggling persists, and document/inspector widths are clamped to their allowed ranges.

- [ ] **Step 2: Run the test and verify RED**

Run: `pnpm test -- src/stores/layout.test.ts`

Expected: FAIL because the current default is expanded and `setInspectorCollapsed` is absent.

- [ ] **Step 3: Implement the layout state**

Change the first-time inspector default to collapsed, add `setInspectorCollapsed`, retain existing persistence keys and clamping rules.

- [ ] **Step 4: Implement the L2 chat shell**

Keep the conversation sidebar and central message column. When collapsed, render a 52px inspector rail with status/toggle slot; when expanded, render the 360px inspector panel and resize handle. Remove the empty top bar and expose `header`, `inspector-rail` and `inspector` slots.

- [ ] **Step 5: Implement the D2 document shell**

Keep the resizable tree sidebar and central paper. Use the same collapsed rail/expanded inspector behavior as chat and keep the document toolbar in one 56–64px row.

- [ ] **Step 6: Run the store test and verify GREEN**

Run: `pnpm test -- src/stores/layout.test.ts`

Expected: all persistence, default and clamp tests PASS.

### Task 4: Chat Workspace Polish

**Files:**
- Modify: `web/src/features/conversation/pages/ChatPage.vue`
- Modify: `web/src/features/conversation/components/ConversationSidebar.vue`
- Modify: `web/src/features/conversation/components/MessageList.vue`
- Modify: `web/src/features/conversation/components/UserMessage.vue`
- Modify: `web/src/features/conversation/components/AssistantMessage.vue`
- Modify: `web/src/features/conversation/components/ChatComposer.vue`
- Modify: `web/src/features/agent-run/components/AgentRunPanel.vue`

**Interfaces:**
- Consumes: L2 shell slots and existing submit/stop/reconnect events.
- Preserves: all existing component props, emits and SSE behavior.

- [ ] **Step 1: Write failing rendering assertions**

Add `web/src/features/conversation/components/chat-visuals.test.ts` that shallow-mounts `UserMessage`, `AssistantMessage` and `ChatComposer`; assert user messages retain bubble semantics, assistant output uses article semantics, and the composer exposes an accessible submit control.

- [ ] **Step 2: Run the test and verify RED**

Run: `pnpm test -- src/features/conversation/components/chat-visuals.test.ts`

Expected: FAIL on the new article and accessible-label requirements.

- [ ] **Step 3: Implement the B2/L2 chat presentation**

Add a meaningful workspace header, branded empty/welcome state, centered readable message column, purple user bubble, unboxed assistant article, compact citations, glass composer, improved reconnect banner, and collapsed Agent status rail. Do not change the question submission or SSE flow.

- [ ] **Step 4: Run the test and verify GREEN**

Run: `pnpm test -- src/features/conversation/components/chat-visuals.test.ts`

Expected: rendering assertions PASS.

### Task 5: Document Workspace Polish

**Files:**
- Modify: `web/src/features/document/pages/DocumentWorkspacePage.vue`
- Modify: `web/src/features/document/components/KnowledgeTree.vue`
- Modify: `web/src/features/document/components/DocumentToolbar.vue`
- Modify: `web/src/features/document/components/DocumentViewer.vue`
- Modify: `web/src/features/document/components/DocumentAiPanel.vue`

**Interfaces:**
- Consumes: D2 shell slots and existing document selection/import/AI events.
- Preserves: document scope labels and existing API behavior.

- [ ] **Step 1: Write failing document rendering assertions**

Add `web/src/features/document/components/document-visuals.test.ts`; verify `DocumentViewer` renders content in an `article`, the toolbar exposes document context, and `DocumentAiPanel` renders an explicit scope label.

- [ ] **Step 2: Run the test and verify RED**

Run: `pnpm test -- src/features/document/components/document-visuals.test.ts`

Expected: FAIL on at least the semantic article requirement.

- [ ] **Step 3: Implement the B2/D2 document presentation**

Create the readable paper surface, refined tree selection, single-row toolbar, collapsed current-document AI rail, explicit scope label, and consistent loading/processing/error shells. Keep sanitized document rendering and all existing events unchanged.

- [ ] **Step 4: Run the test and verify GREEN**

Run: `pnpm test -- src/features/document/components/document-visuals.test.ts`

Expected: rendering assertions PASS.

### Task 6: Knowledge Base and Management Page Unification

**Files:**
- Create: `web/src/components/shared/PageHeader.vue`
- Create: `web/src/components/shared/SurfaceCard.vue`
- Create: `web/src/components/shared/page-primitives.test.ts`
- Modify: `web/src/features/knowledge-base/pages/KnowledgeBaseListPage.vue`
- Modify: `web/src/features/knowledge-base/components/KnowledgeBaseCard.vue`
- Modify: `web/src/features/agent-run/pages/AgentRunListPage.vue`
- Modify: `web/src/features/memory/pages/MemoryPage.vue`
- Modify: `web/src/features/mcp/pages/McpPage.vue`
- Modify: `web/src/features/model-config/pages/ModelConfigPage.vue`
- Modify: `web/src/features/user/pages/ProfilePage.vue`
- Modify: `web/src/features/auth/pages/LoginPage.vue`

**Interfaces:**
- Produces: `PageHeader` slots `eyebrow`, default title, `description`, `actions`.
- Produces: `SurfaceCard` props `interactive?: boolean`, `padding?: 'none' | 'sm' | 'md' | 'lg'`.
- Consumes: Task 1 surface primitives.

- [ ] **Step 1: Write failing primitive tests**

Mount both new shared components and assert semantic header/section elements, actions slot rendering and interactive-state classes.

- [ ] **Step 2: Run the test and verify RED**

Run: `pnpm test -- src/components/shared/page-primitives.test.ts`

Expected: FAIL because both components are absent.

- [ ] **Step 3: Implement shared page primitives**

Create the two components with the exact interfaces above and no data-fetching dependencies.

- [ ] **Step 4: Apply the primitives to knowledge-base and management routes**

Give each page a visible title, description, primary action and stable ready/loading/empty/error surface. Redesign knowledge-base cards around name, description, document count, update time and a clear overflow menu. Use the B2 login atmosphere with a high-contrast form surface.

- [ ] **Step 5: Run the primitive test and verify GREEN**

Run: `pnpm test -- src/components/shared/page-primitives.test.ts`

Expected: all primitive rendering tests PASS.

### Task 7: Full Verification and Cleanup

**Files:**
- Modify only files already touched when verification exposes a concrete issue.

**Interfaces:**
- Verifies the complete redesign without changing backend contracts.

- [ ] **Step 1: Run all automated tests**

Run: `pnpm test`

Expected: all Vitest suites PASS with zero failed tests.

- [ ] **Step 2: Run static verification**

Run: `pnpm typecheck`

Expected: exit code 0 and no TypeScript errors.

Run: `pnpm lint`

Expected: exit code 0 and no ESLint warnings or errors.

- [ ] **Step 3: Run the production build**

Run: `pnpm build`

Expected: exit code 0 and a generated `web/dist` bundle.

- [ ] **Step 4: Inspect the final diff**

Run: `git diff --check` and `git status --short`.

Expected: no whitespace errors; only intentional frontend, test, lockfile and uncommitted design/plan documentation changes appear.

- [ ] **Step 5: Report visual QA limits honestly**

If no authenticated backend fixture is available, report that 1280/1440/1920 runtime visual inspection remains manual. Do not describe static build success as full visual acceptance.
