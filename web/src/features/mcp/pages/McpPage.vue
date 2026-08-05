<script setup lang="ts">
import { ref } from 'vue'
import { useMcpServerList, useDeleteMcpServer, useTestMcpServer, useDiscoverMcpTools, useToggleMcpTool } from '../queries'
import McpServerCard from '../components/McpServerCard.vue'
import McpServerDrawer from '../components/McpServerDrawer.vue'
import McpToolList from '../components/McpToolList.vue'
import ConfirmDialog from '@/components/shared/ConfirmDialog.vue'
import EmptyState from '@/components/shared/EmptyState.vue'
import LoadingSkeleton from '@/components/shared/LoadingSkeleton.vue'
import BaseButton from '@/components/base/BaseButton.vue'
import type { McpServer } from '../types'

const { data: servers, isLoading } = useMcpServerList()
const deleteMutation = useDeleteMcpServer()
const testMutation = useTestMcpServer()
const discoverMutation = useDiscoverMcpTools()
const toggleMutation = useToggleMcpTool()

const showDrawer = ref(false)
const editingServer = ref<McpServer | null>(null)
const showDeleteConfirm = ref(false)
const deletingId = ref<string | null>(null)
const selectedServerId = ref<string | null>(null)
const testResults = ref<Record<string, string>>({})

function handleCreate() {
  editingServer.value = null
  showDrawer.value = true
}

function handleEdit(id: string) {
  const server = servers.value?.find(s => s.id === id)
  if (server) {
    editingServer.value = server
    showDrawer.value = true
  }
}

function handleDelete(id: string) {
  deletingId.value = id
  showDeleteConfirm.value = true
}

async function confirmDelete() {
  if (!deletingId.value) return
  try {
    await deleteMutation.mutateAsync(deletingId.value)
    showDeleteConfirm.value = false
    deletingId.value = null
    if (selectedServerId.value === deletingId.value) {
      selectedServerId.value = null
    }
  } catch {
    // Error handled by mutation
  }
}

async function handleTest(id: string) {
  testResults.value[id] = '测试中...'
  try {
    const result = await testMutation.mutateAsync(id)
    testResults.value[id] = result.status === 'available'
      ? `连接成功 (${result.latency_ms}ms)`
      : `连接失败: ${result.error || '未知错误'}`
  } catch {
    testResults.value[id] = '测试失败'
  }
}

async function handleDiscover(id: string) {
  try {
    const result = await discoverMutation.mutateAsync(id)
    testResults.value[id] = `发现 ${result.tools.length} 个工具`
  } catch {
    testResults.value[id] = '发现失败'
  }
}

async function handleToggle(id: string, enabled: boolean) {
  await toggleMutation.mutateAsync({ toolId: id, enabled })
}

function handleSaved() {
  showDrawer.value = false
  editingServer.value = null
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Header -->
    <div class="flex items-center justify-between border-b border-[var(--memora-border)] px-6 py-4">
      <h1 class="text-xl font-semibold text-[var(--memora-text)]">
        MCP 配置
      </h1>
      <BaseButton @click="handleCreate">
        新建 Server
      </BaseButton>
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto p-6">
      <LoadingSkeleton
        v-if="isLoading"
        type="list"
        :rows="3"
      />

      <EmptyState
        v-else-if="!servers?.length"
        title="暂无 MCP Server"
        description="创建 MCP Server 以启用外部工具集成"
      >
        <template #action>
          <BaseButton @click="handleCreate">
            新建 Server
          </BaseButton>
        </template>
      </EmptyState>

      <div
        v-else
        class="space-y-4"
      >
        <McpServerCard
          v-for="server in servers"
          :key="server.id"
          :server="server"
          @edit="handleEdit"
          @delete="handleDelete"
          @test="handleTest"
          @discover="handleDiscover"
          @toggle="handleToggle"
        />

        <!-- Test results -->
        <div
          v-for="(result, id) in testResults"
          :key="id"
          class="rounded-md bg-[var(--memora-bg)] px-3 py-2 text-sm text-[var(--memora-muted)]"
        >
          {{ result }}
        </div>

        <!-- Tool list for selected server -->
        <div
          v-if="selectedServerId"
          class="mt-6"
        >
          <McpToolList :server-id="selectedServerId" />
        </div>
      </div>
    </div>

    <!-- Drawer -->
    <McpServerDrawer
      v-model:open="showDrawer"
      :editing-server="editingServer"
      @saved="handleSaved"
    />

    <!-- Delete confirmation -->
    <ConfirmDialog
      v-model:open="showDeleteConfirm"
      title="删除 MCP Server"
      message="确定要删除这个 MCP Server 吗？所有工具发现数据将被清除。"
      confirm-text="删除"
      :loading="deleteMutation.isPending.value"
      @confirm="confirmDelete"
    />
  </div>
</template>
