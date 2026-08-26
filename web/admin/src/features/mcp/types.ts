export type TransportType = 'streamable_http' | 'stdio';

export type ConnectionStatus = 'unknown' | 'available' | 'unavailable';

export interface McpServer {
  id: string;
  name: string;
  description: string | null;
  transport: TransportType;
  url?: string;
  headers_masked?: Record<string, string>;
  command?: string;
  args?: string[];
  env_masked?: Record<string, string>;
  auth_masked?: string;
  connect_timeout_ms: number;
  call_timeout_ms: number;
  max_response_bytes: number;
  network_required: boolean;
  enabled: boolean;
  connection_status: ConnectionStatus;
  last_tested_at?: string;
  last_error?: string;
  tools_count: number;
  tools?: McpToolSummary[];
  created_at: string;
}

export interface McpToolSummary {
  id: string;
  tool_name: string;
  description: string | null;
  read_only: boolean;
  enabled: boolean;
}

export interface McpToolDetail extends McpToolSummary {
  input_schema: unknown;
  schema_hash: string;
  discovered_at: string;
  schema_changed_at?: string;
}

export interface McpServerListResponse {
  servers: McpServer[];
}

export interface McpServerDetailResponse {
  server: McpServer;
  tools: McpToolDetail[];
}

export interface McpImportRequest {
  mcpServers: Record<string, McpServerConfig>;
}

export interface McpServerConfig {
  transport?: TransportType;
  url?: string;
  headers?: Record<string, string>;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  cwd?: string;
  description?: string;
  network_required?: boolean;
  connect_timeout_ms?: number;
  call_timeout_ms?: number;
  max_response_bytes?: number;
  enabled?: boolean;
}

export interface McpImportResponse {
  imported: Array<{ server: McpServer; import_warnings: string[] }>;
  failed: Array<{ name: string; error: string; message: string }>;
  summary: { total: number; imported: number; failed: number };
}

export interface McpTestResult {
  success: boolean;
  available: boolean;
  response_time_ms: number;
  error_message?: string;
  last_tested_at: string;
}

export interface McpDiscoverResult {
  tools: McpToolSummary[];
  warnings: string[];
}
