import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const ACCESS_TOKEN_KEY = 'memora.access_token'
const TOKEN_EXPIRES_AT_KEY = 'memora.token_expires_at'

export interface CurrentUser {
  id: string
  username: string
  nickname: string
  email: string
  avatar_url: string | null
  bio?: string
}

export const useAuthStore = defineStore('auth', () => {
  const access_token = ref<string | null>(sessionStorage.getItem(ACCESS_TOKEN_KEY))
  const token_expires_at = ref<number | null>(
    (() => {
      const raw = sessionStorage.getItem(TOKEN_EXPIRES_AT_KEY)
      return raw ? Number(raw) : null
    })(),
  )
  const user = ref<CurrentUser | null>(null)

  const isAuthenticated = computed(() => {
    if (!access_token.value) return false
    if (token_expires_at.value && Date.now() >= token_expires_at.value) return false
    return true
  })

  function setSession(accessToken: string, expiresIn: number, userData: CurrentUser) {
    access_token.value = accessToken
    token_expires_at.value = Date.now() + expiresIn * 1000
    user.value = userData
    sessionStorage.setItem(ACCESS_TOKEN_KEY, accessToken)
    sessionStorage.setItem(TOKEN_EXPIRES_AT_KEY, String(token_expires_at.value))
  }

  function clearSession() {
    access_token.value = null
    token_expires_at.value = null
    user.value = null
    sessionStorage.removeItem(ACCESS_TOKEN_KEY)
    sessionStorage.removeItem(TOKEN_EXPIRES_AT_KEY)
  }

  function restoreSession(): boolean {
    const token = sessionStorage.getItem(ACCESS_TOKEN_KEY)
    const expires = sessionStorage.getItem(TOKEN_EXPIRES_AT_KEY)
    if (!token) return false
    if (expires && Date.now() >= Number(expires)) {
      clearSession()
      return false
    }
    access_token.value = token
    token_expires_at.value = expires ? Number(expires) : null
    return true
  }

  function isExpired(): boolean {
    if (!access_token.value) return true
    if (token_expires_at.value && Date.now() >= token_expires_at.value) return true
    return false
  }

  function setUser(userData: CurrentUser) {
    user.value = userData
  }

  return {
    access_token,
    token_expires_at,
    user,
    isAuthenticated,
    setSession,
    clearSession,
    restoreSession,
    isExpired,
    setUser,
  }
})
