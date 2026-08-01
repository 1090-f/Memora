<script setup lang="ts">
import { ref } from 'vue'

interface Notification {
  id: number
  type: 'success' | 'error' | 'warning' | 'info'
  message: string
}

const notifications = ref<Notification[]>([])
let nextId = 0

function addNotification(type: Notification['type'], message: string, duration = 5000) {
  const id = nextId++
  notifications.value.push({ id, type, message })
  if (duration > 0) {
    setTimeout(() => removeNotification(id), duration)
  }
}

function removeNotification(id: number) {
  notifications.value = notifications.value.filter(n => n.id !== id)
}

defineExpose({ addNotification })
</script>

<template>
  <div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
    <TransitionGroup name="notification">
      <div
        v-for="notification in notifications"
        :key="notification.id"
        :class="[
          'flex items-center gap-2 rounded-lg px-4 py-3 text-sm shadow-lg',
          notification.type === 'success' && 'bg-green-50 text-green-800',
          notification.type === 'error' && 'bg-red-50 text-red-800',
          notification.type === 'warning' && 'bg-yellow-50 text-yellow-800',
          notification.type === 'info' && 'bg-blue-50 text-blue-800',
        ]"
      >
        <span class="flex-1">{{ notification.message }}</span>
        <button
          class="ml-2 opacity-70 hover:opacity-100"
          aria-label="关闭"
          @click="removeNotification(notification.id)"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.notification-enter-active,
.notification-leave-active {
  transition: all 0.3s ease;
}
.notification-enter-from {
  opacity: 0;
  transform: translateX(30px);
}
.notification-leave-to {
  opacity: 0;
  transform: translateX(30px);
}
</style>
