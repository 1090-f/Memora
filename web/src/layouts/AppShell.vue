<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useWorkspaceStore } from '@/stores/workspace'
import { logout } from '@/features/auth/api'
import GlobalSearchDialog from '@/components/shared/GlobalSearchDialog.vue'
import UnsupportedViewport from './UnsupportedViewport.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const workspaceStore = useWorkspaceStore()

const windowWidth = ref(window.innerWidth)
const showSearch = ref(false)
const showUserMenu = ref(false)

const isUnsupported = computed(() => windowWidth.value < 1280)

function handleResize() {
  windowWidth.value = window.innerWidth
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})

// Keyboard shortcut for global search
function handleKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    e.preventDefault()
    showSearch.value = !showSearch.value
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})

const navItems = [
  { name: 'chat', label: '智能问答', path: '/chat', icon: 'chat' },
  { name: 'docs', label: '文档', path: '/kb', icon: 'document' },
  { name: 'runs', label: '运行记录', path: '/runs', icon: 'runs' },
  { name: 'memory', label: '长期记忆', path: '/memories', icon: 'memory' },
  { name: 'mcp', label: 'MCP', path: '/mcp', icon: 'mcp' },
  { name: 'settings', label: '设置', path: '/settings/models', icon: 'settings' },
]

function isActive(path: string): boolean {
  return route.path.startsWith(path)
}

function handleNavigate(path: string) {
  void router.push(path)
}

function handleSearchNavigate(path: string) {
  void router.push(path)
}

async function handleLogout() {
  try {
    await logout()
  } finally {
    authStore.clearSession()
    workspaceStore.clearCurrentKbId()
    void router.replace('/login')
  }
}
</script>

<template>
  <UnsupportedViewport v-if="isUnsupported" />

  <div
    v-else
    class="flex h-screen overflow-hidden bg-[var(--memora-bg)]"
  >
    <!-- Navigation Rail -->
    <nav
      class="flex w-[var(--memora-nav-width)] flex-col items-center border-r border-[var(--memora-border)] bg-[var(--memora-nav)] py-4"
      aria-label="主导航"
    >
      <!-- Logo -->
      <div class="mb-6 flex h-8 w-8 items-center justify-center rounded-md bg-[var(--memora-brand-500)] text-sm font-bold text-white">
        M
      </div>

      <!-- Nav Items -->
      <div class="flex flex-1 flex-col items-center gap-1">
        <button
          v-for="item in navItems"
          :key="item.name"
          :class="[
            'group relative flex h-10 w-10 items-center justify-center rounded-md transition-colors',
            isActive(item.path)
              ? 'bg-[var(--memora-brand-500)] text-white'
              : 'text-gray-400 hover:bg-gray-700 hover:text-white',
          ]"
          :aria-label="item.label"
          :title="item.label"
          @click="handleNavigate(item.path)"
        >
          <!-- Placeholder icons -->
          <svg
            v-if="item.icon === 'chat'"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
          </svg>
          <svg
            v-else-if="item.icon === 'document'"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <svg
            v-else-if="item.icon === 'runs'"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
          <svg
            v-else-if="item.icon === 'memory'"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
          </svg>
          <svg
            v-else-if="item.icon === 'mcp'"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <svg
            v-else-if="item.icon === 'settings'"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>

          <!-- Tooltip -->
          <span class="pointer-events-none absolute left-full ml-2 hidden whitespace-nowrap rounded-md bg-[var(--memora-nav)] px-2 py-1 text-xs text-white group-hover:block">
            {{ item.label }}
          </span>
        </button>
      </div>

      <!-- User Menu -->
      <div class="relative">
        <button
          class="flex h-10 w-10 items-center justify-center rounded-md text-gray-400 hover:bg-gray-700 hover:text-white"
          aria-label="用户菜单"
          @click="showUserMenu = !showUserMenu"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
          </svg>
        </button>

        <div
          v-if="showUserMenu"
          class="absolute bottom-full left-0 mb-2 w-48 rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] py-1 shadow-lg"
        >
          <div class="border-b border-[var(--memora-border)] px-3 py-2">
            <p class="text-sm font-medium text-[var(--memora-text)]">
              {{ authStore.user?.nickname || authStore.user?.username || '用户' }}
            </p>
            <p class="text-xs text-[var(--memora-muted)]">
              {{ authStore.user?.email }}
            </p>
          </div>
          <button
            class="flex w-full items-center px-3 py-2 text-left text-sm text-[var(--memora-text)] hover:bg-[var(--memora-bg)]"
            @click="handleNavigate('/settings/profile'); showUserMenu = false"
          >
            个人设置
          </button>
          <button
            class="flex w-full items-center px-3 py-2 text-left text-sm text-[var(--memora-danger)] hover:bg-[var(--memora-bg)]"
            @click="handleLogout"
          >
            退出登录
          </button>
        </div>
      </div>
    </nav>

    <!-- Main Content -->
    <main class="flex-1 overflow-hidden">
      <slot />
    </main>

    <!-- Global Search Dialog -->
    <GlobalSearchDialog
      :open="showSearch"
      @update:open="showSearch = $event"
      @navigate="handleSearchNavigate"
    />
  </div>
</template>
