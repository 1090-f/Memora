export interface McpServer {
  id: string;
  name: string;
  description: string | null;
  transport: 'streamable_http';
  url: string;
  auth_masked?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}
export interface McpTool {
  id: string;
  server_id: string;
  name: string;
  description: string;
  input_schema: Record<string, unknown>;
  schema_hash: string;
  read_only: boolean;
  enabled: boolean;
  discovered_at: string;
  last_checked_at: string | null;
}
