import { apiRequest } from '@/api/client';
import type { PageResult } from '@/features/knowledge-base/types';
import type { McpServer, McpTool } from './types';

export const listMcpServers = (params: Record<string, unknown> = {}) =>
  apiRequest<PageResult<McpServer>>({ url: '/mcp/servers', params });
export const listMcpTools = (serverId: string) => apiRequest<McpTool[]>({ url: `/mcp/servers/${serverId}/tools` });
export const setMcpToolEnabled = (toolId: string, enabled: boolean) =>
  apiRequest<McpTool>({ url: `/mcp/tools/${toolId}/enabled`, method: 'PATCH', data: { enabled } });
