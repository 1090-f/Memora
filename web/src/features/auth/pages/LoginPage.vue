<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { login } from '@/features/auth/api'
import { useAuthStore } from '@/stores/auth'
import { AppError } from '@/api/errors'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const account = ref('')
const password = ref('')
const loading = ref(false)
const errorMessage = ref('')
const requestId = ref('')

onMounted(() => {
  if (authStore.isAuthenticated) {
    const redirect = (route.query.redirect as string) || '/'
    void router.replace(redirect)
  }
})

async function handleSubmit() {
  if (!account.value || !password.value) {
    errorMessage.value = '请输入账号和密码'
    return
  }

  loading.value = true
  errorMessage.value = ''
  requestId.value = ''

  try {
    const data = await login({
      account: account.value,
      password: password.value,
    })

    authStore.setSession(data.access_token, data.expires_in, data.user)

    const redirect = (route.query.redirect as string) || '/'
    await router.replace(redirect)
  } catch (err) {
    if (err instanceof AppError) {
      errorMessage.value = err.message
      requestId.value = err.requestId || ''
    } else {
      errorMessage.value = '登录失败，请稍后重试'
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-[var(--memora-bg)]">
    <div class="w-full max-w-sm rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-8 shadow-sm">
      <h1 class="mb-6 text-center text-2xl font-semibold text-[var(--memora-text)]">
        Memora
      </h1>

      <form
        class="space-y-4"
        @submit.prevent="handleSubmit"
      >
        <div>
          <label
            for="account"
            class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
          >
            账号
          </label>
          <input
            id="account"
            v-model="account"
            type="text"
            required
            autocomplete="username"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>

        <div>
          <label
            for="password"
            class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
          >
            密码
          </label>
          <input
            id="password"
            v-model="password"
            type="password"
            required
            autocomplete="current-password"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>

        <p
          v-if="errorMessage"
          class="text-sm text-[var(--memora-danger)]"
          role="alert"
        >
          {{ errorMessage }}
          <span
            v-if="requestId"
            class="ml-1 text-xs text-[var(--memora-muted)]"
          >
            ({{ requestId }})
          </span>
        </p>

        <button
          type="submit"
          :disabled="loading"
          class="w-full rounded-md bg-[var(--memora-brand-500)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--memora-brand-600)] disabled:opacity-50"
        >
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>
    </div>
  </div>
</template>
