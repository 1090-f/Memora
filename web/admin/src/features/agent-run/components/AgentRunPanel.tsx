import { Alert, Chip, CircularProgress, Divider, Stack, Typography } from '@mui/material';
import Box from '@mui/material/Box';
import type { AgentRunViewState } from '../types';

/**
 * AgentRunPanel 右侧 Agent 运行面板，展示运行状态、计划步骤列表和工具调用。
 * 计划步骤采用 Trae 风格：待执行用序号圆圈、执行中用高亮序号+旋转加载、完成用绿色对勾。
 */
export function AgentRunPanel({ state }: { state: AgentRunViewState }) {
  return (
    <Stack spacing={2} p={2} sx={{ overflow: 'auto', height: 'calc(100% - 48px)' }}>
      {/* 顶部标题与状态：标题在左侧，状态靠右并为折叠按钮预留空间 */}
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ minHeight: 32, pr: 4, gap: 1 }}
      >
        <Typography variant="h6" fontWeight={700} sx={{ flexShrink: 0 }}>
          Agent 运行
        </Typography>
        <Chip
          size="small"
          label={state.status}
          color={statusColor(state.status)}
          sx={{ flexShrink: 0 }}
        />
      </Stack>

      {/* Router 决策摘要 */}
      {state.router && (
        <Alert severity="info" sx={{ py: 0.5, '& .MuiAlert-message': { fontSize: '0.8rem' } }}>
          <strong>{state.router.execution_mode === 'plan_execute' ? 'Plan-Execute' : 'ReAct'}</strong>
          <br />
          {state.router.reason_summary}
        </Alert>
      )}

      {/* 错误提示 */}
      {state.error && <Alert severity="error">{state.error.message}</Alert>}

      <Divider />

      {/* 计划步骤列表 — Trae 风格 */}
      {state.plan && (
        <Box>
          <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 1.5, display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <rect x="3" y="3" width="18" height="18" rx="2" />
              <line x1="9" y1="12" x2="15" y2="12" />
            </svg>
            执行计划
            <Typography component="span" variant="caption" color="text.secondary" sx={{ ml: 0.5 }}>
              v{state.plan.version}
            </Typography>
          </Typography>
          <Stack spacing={1.5}>
            {state.plan.steps.map((step) => (
              <Box key={step.step_no} sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5 }}>
                {/* 左侧状态指示器 */}
                <StepIndicator status={step.status} stepNo={step.step_no} />
                {/* 右侧步骤内容 */}
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography
                    variant="body2"
                    fontWeight={step.status === 'running' ? 700 : 500}
                    sx={{
                      color: step.status === 'running' ? 'primary.main' : step.status === 'completed' ? 'success.main' : 'text.primary',
                      wordBreak: 'break-word',
                    }}
                  >
                    {step.title}
                  </Typography>
                  {step.status === 'running' && (
                    <Typography variant="caption" color="primary.main" sx={{ display: 'flex', alignItems: 'center', gap: 0.5, mt: 0.25 }}>
                      <CircularProgress size={10} thickness={6} />
                      执行中...
                    </Typography>
                  )}
                  {step.status === 'failed' && step.error_message && (
                    <Typography variant="caption" color="error" sx={{ mt: 0.25, display: 'block' }}>
                      {step.error_message}
                    </Typography>
                  )}
                </Box>
              </Box>
            ))}
          </Stack>
        </Box>
      )}

      {/* 工具调用列表 */}
      {state.tools.length > 0 && (
        <Box>
          <Typography variant="subtitle2" fontWeight={700} sx={{ mb: 1 }}>
            工具调用
          </Typography>
          <Stack spacing={1}>
            {state.tools.map((tool) => (
              <Box key={tool.tool_call_id} sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
                <Box sx={{
                  width: 20, height: 20, borderRadius: '50%', flexShrink: 0, mt: '2px',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  bgcolor: tool.status === 'succeeded' ? 'success.main' : tool.status === 'failed' ? 'error.main' : 'primary.main',
                }}>
                  <Typography variant="caption" sx={{ color: '#fff', fontSize: '0.65rem', lineHeight: 1 }}>
                    {tool.status === 'succeeded' ? '✓' : tool.status === 'failed' ? '✗' : '→'}
                  </Typography>
                </Box>
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography variant="body2" fontWeight={500} sx={{ wordBreak: 'break-word' }}>
                    {tool.tool_name}
                  </Typography>
                  {tool.output_summary && (
                    <Typography variant="caption" color="text.secondary" sx={{
                      display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    }}>
                      {tool.output_summary}
                    </Typography>
                  )}
                </Box>
              </Box>
            ))}
          </Stack>
        </Box>
      )}

      {/* 空状态 */}
      {!state.plan && state.tools.length === 0 && (
        <Typography color="text.secondary" variant="body2" sx={{ textAlign: 'center', py: 4 }}>
          运行后将在这里显示可观察摘要。
        </Typography>
      )}

      {/* Token 用量 */}
      {state.usage && (
        <Box sx={{ mt: 'auto', pt: 1 }}>
          <Divider sx={{ mb: 1 }} />
          <Typography variant="caption" color="text.secondary">
            Token: {state.usage.input_tokens} in / {state.usage.output_tokens} out / {state.usage.total_tokens} total
          </Typography>
        </Box>
      )}
    </Stack>
  );
}

/**
 * StepIndicator 渲染步骤左侧的状态指示器。
 * - pending: 灰色序号圆圈
 * - running: 蓝色高亮序号圆圈 + 旋转边框
 * - completed: 绿色实心对勾圆圈
 * - failed: 红色实心叉号圆圈
 */
function StepIndicator({ status, stepNo }: { status: string; stepNo: number }) {
  if (status === 'completed') {
    return (
      <Box sx={{
        width: 24, height: 24, borderRadius: '50%', flexShrink: 0, mt: '2px',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        bgcolor: 'success.main', color: '#fff', fontSize: '0.75rem', fontWeight: 700,
      }}>
        ✓
      </Box>
    );
  }

  if (status === 'running') {
    return (
      <Box sx={{
        width: 24, height: 24, borderRadius: '50%', flexShrink: 0, mt: '2px',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        border: '2px solid', borderColor: 'primary.main', color: 'primary.main',
        fontSize: '0.75rem', fontWeight: 700, position: 'relative',
      }}>
        <CircularProgress size={24} thickness={2} sx={{ position: 'absolute', color: 'primary.main' }} />
        <span style={{ zIndex: 1 }}>{stepNo}</span>
      </Box>
    );
  }

  if (status === 'failed') {
    return (
      <Box sx={{
        width: 24, height: 24, borderRadius: '50%', flexShrink: 0, mt: '2px',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        bgcolor: 'error.main', color: '#fff', fontSize: '0.75rem', fontWeight: 700,
      }}>
        ✗
      </Box>
    );
  }

  // pending 或其他状态：灰色边框圆圈 + 序号
  return (
    <Box sx={{
      width: 24, height: 24, borderRadius: '50%', flexShrink: 0, mt: '2px',
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      border: '2px solid', borderColor: 'grey.400', color: 'grey.600',
      fontSize: '0.75rem', fontWeight: 600,
    }}>
      {stepNo}
    </Box>
  );
}

/** statusColor 根据 Agent 运行状态返回对应的 Chip 颜色。 */
function statusColor(status: string): 'default' | 'primary' | 'success' | 'error' | 'warning' | 'info' {
  switch (status) {
    case 'running': return 'primary';
    case 'completed': return 'success';
    case 'failed': return 'error';
    case 'cancelled': return 'warning';
    case 'queued': return 'info';
    default: return 'default';
  }
}