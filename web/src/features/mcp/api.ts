import { request } from '@/api/client'
import type {
  McpServer,
  CreateMcpServerRequest,
  UpdateMcpServerRequest,
  McpTestResult,
  McpTool,
  DiscoverResponse,
} from './types'

// Server APIs
export function listMcpServers(): Promise<McpServer[]> {
  return request<McpServer[]>('/mcp/servers')
}

export function getMcpServer(id: string): Promise<McpServer> {
  return request<McpServer>(`/mcp/servers/${id}`)
}

export function createMcpServer(data: CreateMcpServerRequest): Promise<McpServer> {
  return request<McpServer>('/mcp/servers', {
    method: 'POST',
    body: data,
  })
}

export function updateMcpServer(id: string, data: UpdateMcpServerRequest): Promise<McpServer> {
  return request<McpServer>(`/mcp/servers/${id}`, {
    method: 'PATCH',
    body: data,
  })
}

export function deleteMcpServer(id: string): Promise<void> {
  return request<void>(`/mcp/servers/${id}`, {
    method: 'DELETE',
  })
}

export function testMcpServer(id: string): Promise<McpTestResult> {
  return request<McpTestResult>(`/mcp/servers/${id}/test`, {
    method: 'POST',
  })
}

export function discoverMcpTools(id: string): Promise<DiscoverResponse> {
  return request<DiscoverResponse>(`/mcp/servers/${id}/discover`, {
    method: 'POST',
  })
}

// Tool APIs
export function listMcpTools(serverId: string): Promise<McpTool[]> {
  return request<McpTool[]>(`/mcp/servers/${serverId}/tools`)
}

export function getMcpTool(toolId: string): Promise<McpTool> {
  return request<McpTool>(`/mcp/tools/${toolId}`)
}

export function toggleMcpTool(toolId: string, enabled: boolean): Promise<void> {
  return request<void>(`/mcp/tools/${toolId}/enabled`, {
    method: 'PATCH',
    body: { enabled },
  })
}

// Knowledge base tool authorization
export function grantToolToKnowledgeBase(kbId: string, toolId: string): Promise<void> {
  return request<void>(`/knowledge-bases/${kbId}/agent-config/mcp-tools/${toolId}`, {
    method: 'PUT',
    body: { enabled: true },
  })
}

export function revokeToolFromKnowledgeBase(kbId: string, toolId: string): Promise<void> {
  return request<void>(`/knowledge-bases/${kbId}/agent-config/mcp-tools/${toolId}`, {
    method: 'DELETE',
  })
}
