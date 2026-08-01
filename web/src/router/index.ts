import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getCurrentUser } from '@/features/user/api'
import { configureAuthTransport } from '@/api/client'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/login',
    },
    {
      path: '/login',
      name: 'login',
      component: () => import('@/features/auth/pages/LoginPage.vue'),
    },
    {
      path: '/knowledge-bases',
      component: () => import('@/layouts/AppShell.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'knowledge-bases',
          component: () => import('@/features/knowledge-base/pages/KnowledgeBaseListPage.vue'),
        },
      ],
    },
    {
      path: '/kb/:kbId/docs',
      component: () => import('@/layouts/AppShell.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'document-workspace',
          component: () => import('@/features/document/pages/DocumentWorkspacePage.vue'),
        },
        {
          path: ':documentId',
          name: 'document-detail',
          component: () => import('@/features/document/pages/DocumentWorkspacePage.vue'),
        },
      ],
    },
    {
      path: '/kb/:kbId/search-test',
      component: () => import('@/layouts/AppShell.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'search-test',
          component: () => import('@/features/search/pages/SearchTestPage.vue'),
        },
      ],
    },
    {
      path: '/kb/:kbId/settings',
      component: () => import('@/layouts/AppShell.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: 'kb-settings',
          component: () => import('@/features/knowledge-base/pages/KnowledgeBaseSettingsPage.vue'),
        },
      ],
    },
    {
      path: '/settings',
      component: () => import('@/layouts/AppShell.vue'),
      meta: { requiresAuth: true },
      children: [
        {
          path: 'profile',
          name: 'profile',
          component: () => import('@/features/user/pages/ProfilePage.vue'),
        },
        {
          path: 'models',
          name: 'model-config',
          component: () => import('@/features/model-config/pages/ModelConfigPage.vue'),
        },
      ],
    },
  ],
})

// Configure auth transport
configureAuthTransport({
  getAccessToken: () => {
    const authStore = useAuthStore()
    return authStore.access_token
  },
  onUnauthorized: () => {
    const authStore = useAuthStore()
    authStore.clearSession()
    void router.replace({
      name: 'login',
      query: { redirect: router.currentRoute.value.fullPath },
    })
  },
})

let sessionRestored = false

router.beforeEach(async (to) => {
  const authStore = useAuthStore()

  // Allow login page without auth
  if (to.name === 'login') {
    return true
  }

  // Restore session once on first navigation
  if (!sessionRestored) {
    sessionRestored = true
    const restored = authStore.restoreSession()
    if (!restored) {
      return {
        name: 'login',
        query: { redirect: to.fullPath },
      }
    }
  }

  // Check auth - if route has requiresAuth or is a child of a requiresAuth route
  const requiresAuth = to.matched.some(record => record.meta.requiresAuth)
  if (requiresAuth && !authStore.isAuthenticated) {
    return {
      name: 'login',
      query: { redirect: to.fullPath },
    }
  }

  // Load current user if not loaded
  if (authStore.isAuthenticated && !authStore.user) {
    try {
      const user = await getCurrentUser()
      authStore.setUser(user)
    } catch {
      authStore.clearSession()
      return {
        name: 'login',
        query: { redirect: to.fullPath },
      }
    }
  }

  return true
})

export default router
