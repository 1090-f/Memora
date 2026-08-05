# ChatGPT-Style Global Sidebar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the oversized Notion-style app rail with a compact ChatGPT-style global sidebar containing navigation, conversation history, and the account footer.

**Architecture:** `AppShell.vue` becomes the sole owner of the desktop navigation and history layout. It reuses the existing conversation query mutations through a focused `ConversationSidebar` presentation, while `ChatPage.vue` becomes a single-pane conversation workspace so history is not duplicated.

**Tech Stack:** Vue 3 Composition API, TypeScript, Pinia, TanStack Vue Query, Tailwind CSS, Vitest.

## Global Constraints

- Reuse the existing conversation API queries and routes; do not change backend contracts.
- Keep the sidebar keyboard-accessible, with semantic buttons and existing aria labels.
- Preserve the current sidebar-collapse state in `useLayoutStore`.
- Do not overwrite unrelated uncommitted work.

---

### Task 1: Define the compact global-sidebar visual contract

**Files:**
- Modify: `web/src/layouts/AppShell.test.ts`
- Modify: `web/src/layouts/AppShell.vue`

**Interfaces:**
- Consumes: `APP_NAV_ITEMS`, `useLayoutStore`, current-route state.
- Produces: a 260px app sidebar with `data-testid="global-sidebar"`, a compact nav, and a scrollable `data-testid="global-conversation-history"` slot.

- [ ] **Step 1: Write the failing test**

```ts
it('renders a compact ChatGPT-style global sidebar with a conversation-history region', async () => {
  const { wrapper } = await mountAppShell('/knowledge-bases')

  expect(wrapper.get('[data-testid="global-sidebar"]').classes()).toContain('w-[260px]')
  expect(wrapper.get('nav').classes()).toContain('gap-1')
  expect(wrapper.get('[data-testid="global-conversation-history"]')).toBeTruthy()
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test src/layouts/AppShell.test.ts`

Expected: FAIL because the compact sidebar markers and history region do not exist.

- [ ] **Step 3: Write minimal implementation**

```vue
<aside data-testid="global-sidebar" class="flex w-[260px] shrink-0 flex-col border-r border-black/10 bg-[#f9f9f9]">
  <nav class="flex flex-col gap-1 px-2">...</nav>
  <section data-testid="global-conversation-history" class="min-h-0 flex-1 overflow-y-auto">...</section>
</aside>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm test src/layouts/AppShell.test.ts`

Expected: PASS.

### Task 2: Adapt conversation history for the global sidebar

**Files:**
- Modify: `web/src/features/conversation/components/conversation-sidebar.test.ts`
- Modify: `web/src/features/conversation/components/ConversationSidebar.vue`

**Interfaces:**
- Consumes: `knowledgeBaseId`, optional `selectedId`, `useConversationList`, `useCreateConversation`, `useDeleteConversation`.
- Produces: `compact` presentation with `data-new-conversation`, a recent-history list, selected and hover styles, and `select` emissions.

- [ ] **Step 1: Write the failing test**

```ts
it('renders the compact recent-history layout when used by the global sidebar', () => {
  const wrapper = mount(ConversationSidebar, {
    props: { knowledgeBaseId: 'kb-1', compact: true },
  })

  expect(wrapper.get('[data-testid="conversation-history"]')).toBeTruthy()
  expect(wrapper.get('[data-new-conversation]').classes()).toContain('rounded-lg')
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test src/features/conversation/components/conversation-sidebar.test.ts`

Expected: FAIL because `compact` and its history marker are absent.

- [ ] **Step 3: Write minimal implementation**

```ts
const props = withDefaults(defineProps<{
  knowledgeBaseId: string
  selectedId?: string | null
  compact?: boolean
}>(), { compact: false })
```

```vue
<section data-testid="conversation-history" class="min-h-0 flex-1 overflow-y-auto">
  <p class="px-3 pb-1 text-xs font-medium text-[#666]">最近</p>
  <!-- compact conversation buttons -->
</section>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm test src/features/conversation/components/conversation-sidebar.test.ts`

Expected: PASS.

### Task 3: Connect global history and remove the duplicate chat-page rail

**Files:**
- Modify: `web/src/layouts/AppShell.vue`
- Modify: `web/src/features/conversation/pages/ChatPage.vue`
- Modify: `web/src/layouts/AppShell.test.ts`

**Interfaces:**
- Consumes: route parameter `kbId` and optional `conversationId`.
- Produces: sidebar history that navigates to `/chat/:kbId/:conversationId`; a chat page with only the message workspace.

- [ ] **Step 1: Write the failing test**

```ts
it('does not render a second conversation sidebar inside the chat page', () => {
  const source = readFileSync(new URL('../features/conversation/pages/ChatPage.vue', import.meta.url), 'utf8')
  expect(source).not.toContain('<ConversationSidebar')
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm test src/layouts/AppShell.test.ts`

Expected: FAIL because `ChatPage.vue` still renders `ConversationSidebar`.

- [ ] **Step 3: Write minimal implementation**

```vue
<ConversationSidebar
  v-if="route.params.kbId"
  compact
  :knowledge-base-id="route.params.kbId as string"
  :selected-id="route.params.conversationId as string | undefined"
  @select="handleConversationSelect"
/>
```

```vue
<!-- ChatPage.vue -->
<main class="flex min-w-0 flex-1 flex-col bg-[var(--memora-surface)]">...</main>
```

- [ ] **Step 4: Run focused tests and typecheck**

Run: `pnpm test src/layouts/AppShell.test.ts src/features/conversation/components/conversation-sidebar.test.ts && pnpm typecheck`

Expected: PASS with no type errors.

### Task 4: Verify the completed visual integration

**Files:**
- Modify: no production files expected.

- [ ] **Step 1: Run the full frontend test suite**

Run: `pnpm test`

Expected: PASS.

- [ ] **Step 2: Build the web application**

Run: `pnpm build`

Expected: successful typecheck and Vite build.
