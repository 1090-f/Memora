import AddOutlined from '@mui/icons-material/AddOutlined';
import DeleteOutlineOutlined from '@mui/icons-material/DeleteOutlineOutlined';
import SearchOutlined from '@mui/icons-material/SearchOutlined';
import ScienceOutlined from '@mui/icons-material/ScienceOutlined';
import { Alert, Button, Card, CardContent, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Divider, IconButton, Switch, Stack, TextField, Typography } from '@mui/material';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { capabilities, type CapabilityStatus } from '@/app/capabilities';
import { queryKeys } from '@/api/queryKeys';
import { EmptyState } from '@/components/shared/EmptyState';
import { ActionNotice } from '@/components/shared/ActionNotice';
import { ErrorState } from '@/components/shared/ErrorState';
import { LoadingState } from '@/components/shared/LoadingState';
import { UnavailableState } from '@/components/shared/UnavailableState';
import { deleteMcpServer, discoverMcpTools, importMcpServers, listMcpServers, setMcpServerEnabled, setMcpToolEnabled, testMcpServer } from '../api';
import type {
  McpImportResponse,
  McpServer,
  McpServerConfig,
} from '../types';

const statusLabel: Record<McpServer['connection_status'], string> = {
  unknown: '未测试',
  available: '可用',
  unavailable: '不可用',
};

function ServerCard({ server, onDelete, onTest, onDiscover, onToggleServer, onToggleTool }: {
  server: McpServer;
  onDelete: (id: string) => void;
  onTest: (id: string) => void;
  onDiscover: (id: string) => void;
  onToggleServer: (id: string, enabled: boolean) => void;
  onToggleTool: (id: string, enabled: boolean) => void;
}) {
  return (
    <Card variant="outlined">
      <CardContent>
        <Stack direction="row" alignItems="flex-start" spacing={2}>
          <Stack spacing={0.75} sx={{ flexGrow: 1, minWidth: 0 }}>
            <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
              <Typography variant="h6">{server.name}</Typography>
              <Chip size="small" label={server.transport === 'stdio' ? 'stdio' : 'Streamable HTTP'} />
              <Chip size="small" label={server.network_required ? '需联网' : '本地运行'} color={server.network_required ? 'info' : 'default'} variant="outlined" />
              <Chip size="small" color={server.connection_status === 'available' ? 'success' : server.connection_status === 'unavailable' ? 'error' : 'default'} label={statusLabel[server.connection_status]} />
            </Stack>
            <Typography color="text.secondary" sx={{ overflowWrap: 'anywhere' }}>
              {server.transport === 'stdio' ? server.command : server.url}
            </Typography>
            {server.description && <Typography color="text.secondary">{server.description}</Typography>}
            <Stack direction="row" spacing={1} flexWrap="wrap">
              <Chip size="small" variant="outlined" label={`${server.tools_count} 个工具`} />
              {server.auth_masked && <Chip size="small" variant="outlined" label={`认证 ${server.auth_masked}`} />}
              {server.last_error && <Chip size="small" color="error" variant="outlined" label={server.last_error} />}
            </Stack>
          </Stack>
          <Stack direction="row" alignItems="center" spacing={0.5}>
            <IconButton aria-label="测试连接" onClick={() => onTest(server.id)}><ScienceOutlined /></IconButton>
            <IconButton aria-label="发现工具" onClick={() => onDiscover(server.id)}><SearchOutlined /></IconButton>
            <Stack direction="row" alignItems="center" spacing={0.5}><Switch checked={server.enabled} onChange={(event) => onToggleServer(server.id, event.target.checked)} inputProps={{ 'aria-label': `${server.name} 启用` }} /><Typography variant="caption">启用</Typography></Stack>
            <IconButton aria-label="删除" color="error" onClick={() => onDelete(server.id)}><DeleteOutlineOutlined /></IconButton>
          </Stack>
        </Stack>
        {server.tools && server.tools.length > 0 && (
          <Stack spacing={1} mt={2}>
            <Divider />
            <Typography variant="subtitle2">已发现工具</Typography>
            {server.tools.map((tool) => <ToolRow key={tool.id} tool={tool} onToggle={onToggleTool} />)}
          </Stack>
        )}
      </CardContent>
    </Card>
  );
}

function ToolRow({ tool, onToggle }: { tool: NonNullable<McpServer['tools']>[number]; onToggle: (id: string, enabled: boolean) => void }) {
  return (
    <Stack direction="row" alignItems="center" spacing={1}>
      <Stack sx={{ flexGrow: 1, minWidth: 0 }}>
        <Stack direction="row" spacing={1} alignItems="center"><Typography variant="body2" fontWeight={650}>{tool.tool_name}</Typography><Chip size="small" label={tool.read_only ? '只读' : '写入'} color={tool.read_only ? 'success' : 'warning'} /></Stack>
        {tool.description && <Typography variant="caption" color="text.secondary" noWrap>{tool.description}</Typography>}
      </Stack>
      <Switch size="small" checked={tool.enabled} disabled={!tool.read_only} onChange={(event) => onToggle(tool.id, event.target.checked)} inputProps={{ 'aria-label': `${tool.tool_name} 启用` }} />
    </Stack>
  );
}

function ImportDialog({ open, onClose, onImported }: { open: boolean; onClose: () => void; onImported: (message: string) => void }) {
  const [json, setJson] = useState('{\n  "mcpServers": {\n    "baidu-search": {\n      "command": "npx",\n      "args": ["-y", "baidu-search-mcp"]\n    }\n  }\n}');
  const [failedServers, setFailedServers] = useState<McpImportResponse['failed']>([]);
  const mutation = useMutation({ mutationFn: importMcpServers });

  async function handleSubmit() {
    try {
      const result = await mutation.mutateAsync(JSON.parse(json) as { mcpServers: Record<string, McpServerConfig> });
      setFailedServers(result.failed);
      if (result.failed.length === 0) {
        onImported(`导入完成：成功 ${result.summary.imported} 个`);
        onClose();
        return;
      }
      onImported(`导入完成：成功 ${result.summary.imported} 个，失败 ${result.summary.failed} 个，请查看下方失败原因`);
    } catch {
      // Dialog displays the mutation or JSON error below.
    }
  }

  function handleClose() {
    setFailedServers([]);
    mutation.reset();
    onClose();
  }

  let jsonError = '';
  try { JSON.parse(json); } catch { jsonError = '请输入有效的 JSON'; }

  return <Dialog open={open} onClose={handleClose} fullWidth maxWidth="md">
    <DialogTitle>导入 MCP Server</DialogTitle>
    <DialogContent>
      <Typography color="text.secondary" mb={2}>支持 streamable_http 和 stdio 配置。敏感 Header、环境变量只会在服务端加密保存。stdio 本地服务默认不联网；如需联网请设置 {"\"network_required\": true"}，远程 HTTP 服务默认需要联网。</Typography>
      <TextField fullWidth multiline minRows={12} label="MCP 配置 JSON" value={json} onChange={(event) => setJson(event.target.value)} error={!!jsonError} helperText={jsonError || '格式：{ "mcpServers": { "名称": { "command": "npx", "args": ["-y", "包名"], "network_required": true } } }'} inputProps={{ spellCheck: false }} />
      {mutation.error && <Alert severity="error" sx={{ mt: 2 }}>{(mutation.error as Error).message}</Alert>}
      {failedServers.length > 0 && (
        <Stack spacing={1} mt={2}>
          <Alert severity="warning">以下 Server 导入失败：</Alert>
          {failedServers.map((failed) => (
            <Alert key={failed.name} severity="error">
              <Typography fontWeight={600}>{failed.name}</Typography>
              <Typography variant="body2">{failed.message}</Typography>
            </Alert>
          ))}
        </Stack>
      )}
    </DialogContent>
    <DialogActions><Button onClick={handleClose}>{failedServers.length > 0 ? '关闭' : '取消'}</Button><Button variant="contained" onClick={() => void handleSubmit()} disabled={!!jsonError || mutation.isPending}>导入</Button></DialogActions>
  </Dialog>;
}

export function McpPageContent({ status }: { status: CapabilityStatus }) {
  const enabled = status === 'available';
  const queryClient = useQueryClient();
  const [importOpen, setImportOpen] = useState(false);
  const [notice, setNotice] = useState('');
  const [actionError, setActionError] = useState<Error | null>(null);
  const query = useQuery({ queryKey: queryKeys.mcpServers, queryFn: listMcpServers, enabled });
  const action = useMutation({
    mutationFn: async ({ type, id }: { type: 'delete' | 'test' | 'discover'; id: string }) => {
      if (type === 'delete') return deleteMcpServer(id);
      if (type === 'test') return testMcpServer(id);
      return discoverMcpTools(id);
    },
    onSuccess: (result, variables) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.mcpServers });
      if (variables.type === 'test') {
        const testResult = result as Awaited<ReturnType<typeof testMcpServer>>;
        setNotice(testResult.available ? `连接成功，响应 ${testResult.response_time_ms}ms` : `连接失败：${testResult.error_message || '未知错误'}`);
      }
      if (variables.type === 'discover') {
        const discoverResult = result as Awaited<ReturnType<typeof discoverMcpTools>>;
        const warningText = discoverResult.warnings.length ? `，警告：${discoverResult.warnings.join('；')}` : '';
        setNotice(`发现 ${discoverResult.tools.length} 个工具${warningText}`);
      }
    },
    onError: (error) => setActionError(error as Error),
  });
  const toggleTool = useMutation({ mutationFn: ({ id, enabled: value }: { id: string; enabled: boolean }) => setMcpToolEnabled(id, value), onSuccess: () => void queryClient.invalidateQueries({ queryKey: queryKeys.mcpServers }), onError: (error) => setActionError(error as Error) });
  const toggleServer = useMutation({ mutationFn: ({ id, enabled: value }: { id: string; enabled: boolean }) => setMcpServerEnabled(id, value), onSuccess: () => void queryClient.invalidateQueries({ queryKey: queryKeys.mcpServers }), onError: (error) => setActionError(error as Error) });

  if (!enabled) return <Stack spacing={3}>
    <Stack direction="row" alignItems="center"><Typography component="h2" variant="h5" fontWeight={750} sx={{ flexGrow: 1 }}>MCP 工具</Typography><Button variant="contained" disabled>新增 MCP 服务</Button></Stack>
    <UnavailableState title="MCP 后端待接入" description="后端接口已完成；启用此能力后可导入 Server、测试连接、发现工具并切换只读工具。" capability="mcp" />
  </Stack>;

  return <Stack spacing={3}>
    <Stack direction="row" alignItems="center"><BoxTitle /><Button variant="contained" startIcon={<AddOutlined />} onClick={() => setImportOpen(true)}>导入 MCP 服务</Button></Stack>
    <ActionNotice
      message={actionError ? `操作失败：${actionError.message}` : notice}
      severity={actionError || notice.startsWith('连接失败') ? 'error' : 'success'}
      onClose={() => { setNotice(''); setActionError(null); }}
    />
    {query.isPending && <LoadingState label="正在加载 MCP 服务" />}
    {query.error && <ErrorState error={query.error as Error} onRetry={() => void query.refetch()} />}
    {!query.isPending && !query.error && query.data?.servers.length === 0 && <EmptyState title="暂无 MCP Server" description="导入 Claude Desktop、Cursor 或 Trae 格式的 MCP 配置。" action={<Button variant="contained" onClick={() => setImportOpen(true)}>导入配置</Button>} />}
    {query.data?.servers.map((server) => <ServerCard key={server.id} server={server} onDelete={(id) => void action.mutateAsync({ type: 'delete', id })} onTest={(id) => void action.mutateAsync({ type: 'test', id })} onDiscover={(id) => void action.mutateAsync({ type: 'discover', id })} onToggleServer={(id, value) => toggleServer.mutate({ id, enabled: value })} onToggleTool={(id, value) => toggleTool.mutate({ id, enabled: value })} />)}
    <ImportDialog open={importOpen} onClose={() => setImportOpen(false)} onImported={(message) => { setNotice(message); void queryClient.invalidateQueries({ queryKey: queryKeys.mcpServers }); }} />
  </Stack>;
}

function BoxTitle() { return <Stack sx={{ flexGrow: 1 }}><Typography component="h2" variant="h5" fontWeight={750}>MCP 工具</Typography><Typography color="text.secondary">管理 MCP Server，并查看和启用已发现的工具。</Typography></Stack>; }

export function McpPage() { return <McpPageContent status={capabilities.mcp} />; }
