<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { updateProfile, changePassword } from '@/features/user/api'
import { AppError } from '@/api/errors'

const authStore = useAuthStore()

// Profile form
const nickname = ref('')
const email = ref('')
const bio = ref('')
const profileLoading = ref(false)
const profileSuccess = ref(false)
const profileError = ref('')
const profileRequestId = ref('')

// Password form
const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordLoading = ref(false)
const passwordSuccess = ref(false)
const passwordError = ref('')
const passwordRequestId = ref('')

onMounted(() => {
  if (authStore.user) {
    nickname.value = authStore.user.nickname || ''
    email.value = authStore.user.email || ''
    bio.value = authStore.user.bio || ''
  }
})

async function handleProfileSubmit() {
  profileLoading.value = true
  profileSuccess.value = false
  profileError.value = ''
  profileRequestId.value = ''

  try {
    const updated = await updateProfile({
      nickname: nickname.value,
      email: email.value,
      bio: bio.value,
    })
    authStore.setUser(updated)
    profileSuccess.value = true
  } catch (err) {
    if (err instanceof AppError) {
      profileError.value = err.message
      profileRequestId.value = err.requestId || ''
    } else {
      profileError.value = '修改失败，请稍后重试'
    }
  } finally {
    profileLoading.value = false
  }
}

async function handlePasswordSubmit() {
  if (newPassword.value !== confirmPassword.value) {
    passwordError.value = '两次输入的密码不一致'
    return
  }

  if (newPassword.value.length < 6) {
    passwordError.value = '新密码至少 6 位'
    return
  }

  passwordLoading.value = true
  passwordSuccess.value = false
  passwordError.value = ''
  passwordRequestId.value = ''

  try {
    await changePassword({
      old_password: oldPassword.value,
      new_password: newPassword.value,
    })
    passwordSuccess.value = true
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  } catch (err) {
    if (err instanceof AppError) {
      passwordError.value = err.message
      passwordRequestId.value = err.requestId || ''
    } else {
      passwordError.value = '修改密码失败，请稍后重试'
    }
  } finally {
    passwordLoading.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl p-8">
    <h1 class="mb-8 text-2xl font-semibold text-[var(--memora-text)]">
      个人设置
    </h1>

    <!-- Profile Section -->
    <section class="mb-8 rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-6">
      <h2 class="mb-4 text-lg font-medium text-[var(--memora-text)]">
        基本信息
      </h2>

      <form
        class="space-y-4"
        @submit.prevent="handleProfileSubmit"
      >
        <div>
          <label
            for="nickname"
            class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
          >
            昵称
          </label>
          <input
            id="nickname"
            v-model="nickname"
            type="text"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>

        <div>
          <label
            for="email"
            class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
          >
            邮箱
          </label>
          <input
            id="email"
            v-model="email"
            type="email"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>

        <div>
          <label
            for="bio"
            class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
          >
            简介
          </label>
          <textarea
            id="bio"
            v-model="bio"
            rows="3"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          />
        </div>

        <p
          v-if="profileError"
          class="text-sm text-[var(--memora-danger)]"
          role="alert"
        >
          {{ profileError }}
          <span
            v-if="profileRequestId"
            class="ml-1 text-xs text-[var(--memora-muted)]"
          >
            ({{ profileRequestId }})
          </span>
        </p>

        <p
          v-if="profileSuccess"
          class="text-sm text-[var(--memora-success)]"
        >
          保存成功
        </p>

        <button
          type="submit"
          :disabled="profileLoading"
          class="rounded-md bg-[var(--memora-brand-500)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--memora-brand-600)] disabled:opacity-50"
        >
          {{ profileLoading ? '保存中...' : '保存' }}
        </button>
      </form>
    </section>

    <!-- Password Section -->
    <section class="rounded-lg border border-[var(--memora-border)] bg-[var(--memora-surface)] p-6">
      <h2 class="mb-4 text-lg font-medium text-[var(--memora-text)]">
        修改密码
      </h2>

      <form
        class="space-y-4"
        @submit.prevent="handlePasswordSubmit"
      >
        <div>
          <label
            for="oldPassword"
            class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
          >
            当前密码
          </label>
          <input
            id="oldPassword"
            v-model="oldPassword"
            type="password"
            required
            autocomplete="current-password"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>

        <div>
          <label
            for="newPassword"
            class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
          >
            新密码
          </label>
          <input
            id="newPassword"
            v-model="newPassword"
            type="password"
            required
            autocomplete="new-password"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>

        <div>
          <label
            for="confirmPassword"
            class="mb-1 block text-sm font-medium text-[var(--memora-text)]"
          >
            确认新密码
          </label>
          <input
            id="confirmPassword"
            v-model="confirmPassword"
            type="password"
            required
            autocomplete="new-password"
            class="w-full rounded-md border border-[var(--memora-border)] bg-[var(--memora-surface)] px-3 py-2 text-sm outline-none focus:border-[var(--memora-brand-500)] focus:ring-1 focus:ring-[var(--memora-brand-500)]"
          >
        </div>

        <p
          v-if="passwordError"
          class="text-sm text-[var(--memora-danger)]"
          role="alert"
        >
          {{ passwordError }}
          <span
            v-if="passwordRequestId"
            class="ml-1 text-xs text-[var(--memora-muted)]"
          >
            ({{ passwordRequestId }})
          </span>
        </p>

        <p
          v-if="passwordSuccess"
          class="text-sm text-[var(--memora-success)]"
        >
          密码修改成功
        </p>

        <button
          type="submit"
          :disabled="passwordLoading"
          class="rounded-md bg-[var(--memora-brand-500)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--memora-brand-600)] disabled:opacity-50"
        >
          {{ passwordLoading ? '修改中...' : '修改密码' }}
        </button>
      </form>
    </section>
  </div>
</template>
