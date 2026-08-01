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
      path: '/settings/profile',
      name: 'profile',
      component: () => import('@/features/user/pages/ProfilePage.vue'),
      meta: { requiresAuth: true },
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

  // Check auth
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
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
