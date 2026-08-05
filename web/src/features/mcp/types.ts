export type TransportType = 'streamable_http'

export type ServerStatus = 'available' | 'unavailable' | 'testing'

export type ToolStatus = 'running' | 'succeeded' | 'failed' | 'timeout' | 'cancelled'

export interface McpServer {
  id: string
  name: string
  description: string | null
  transport: TransportType
  url: string
  headers: Record<string, string> | null
  auth_configured: boolean
  connect_timeout_ms: number
  call_timeout_ms: number
  max_response_bytes: number
  enabled: boolean
  status: ServerStatus
  last_tested_at: string | null
  created_at: string
  updated_at: string
}

export interface CreateMcpServerRequest {
  name: string
  description?: string
  transport: TransportType
  url: string
  headers?: Record<string, string>
  auth?: {
    type: 'bearer'
    token: string
  }
  connect_timeout_ms?: number
  call_timeout_ms?: number
  max_response_bytes?: number
  enabled?: boolean
}

export interface UpdateMcpServerRequest {
  name?: string
  description?: string
  url?: string
  headers?: Record<string, string>
  auth?: {
    type: 'bearer'
    token: string
  }
  connect_timeout_ms?: number
  call_timeout_ms?: number
  max_response_bytes?: number
  enabled?: boolean
}

export interface McpTestResult {
  status: 'available' | 'unavailable'
  auth_ok: boolean
  latency_ms: number
  error: string | null
}

export interface McpTool {
  id: string
  server_id: string
  name: string
  description: string | null
  input_schema: Record<string, unknown> | null
  schema_hash: string
  read_only: boolean
  enabled: boolean
  discovered_at: string
  last_checked_at: string | null
}

export interface DiscoverResponse {
  server_id: string
  discovered_at: string
  tools: Array<{
    id: string
    name: string
    description: string
    schema_hash: string
    read_only: boolean
    enabled: boolean
  }>
}

export const mcpKeys = {
  all: ['mcp'] as const,
  servers: ['mcp', 'servers'] as const,
  server: (id: string) => ['mcp', 'servers', id] as const,
  tools: (serverId: string) => ['mcp', 'servers', serverId, 'tools'] as const,
  tool: (toolId: string) => ['mcp', 'tools', toolId] as const,
}
