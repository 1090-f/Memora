import { apiRequest } from '@/api/client';
import type {
  McpDiscoverResult,
  McpImportRequest,
  McpImportResponse,
  McpServerDetailResponse,
  McpServerListResponse,
  McpTestResult,
} from './types';

export const listMcpServers = () => apiRequest<McpServerListResponse>({ url: '/mcp/servers' });

export const getMcpServer = (serverId: string) =>
  apiRequest<McpServerDetailResponse>({ url: `/mcp/servers/${serverId}` });

export const importMcpServers = (data: McpImportRequest) =>
  apiRequest<McpImportResponse>({ url: '/mcp/servers/import', method: 'POST', data });

export const deleteMcpServer = (serverId: string) =>
  apiRequest<{ deleted: boolean }>({ url: `/mcp/servers/${serverId}`, method: 'DELETE' });

export const testMcpServer = (serverId: string) =>
  apiRequest<McpTestResult>({ url: `/mcp/servers/${serverId}/test`, method: 'POST' });

export const discoverMcpTools = (serverId: string) =>
  apiRequest<McpDiscoverResult>({ url: `/mcp/servers/${serverId}/discover`, method: 'POST' });

export const setMcpToolEnabled = (toolId: string, enabled: boolean) =>
  apiRequest<{ enabled: boolean }>({ url: `/mcp/tools/${toolId}`, method: 'PATCH', data: { enabled } });
