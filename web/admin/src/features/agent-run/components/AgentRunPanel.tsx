import CheckCircleOutlined from '@mui/icons-material/CheckCircleOutlined';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import SmartToyOutlined from '@mui/icons-material/SmartToyOutlined';
import SyncOutlined from '@mui/icons-material/SyncOutlined';
import BuildOutlined from '@mui/icons-material/BuildOutlined';
import { Alert, Box, Button, Chip, Divider, Paper, Stack, Typography } from '@mui/material';
import type { AgentRunViewState } from '../types';

const statusLabel = {
  idle: '等待中',
  queued: '排队中',
  running: '运行中',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
} as const;

function citationTitle(citation: Record<string, unknown>, index: number) {
  const value = citation.document_title || citation.title || citation.url;
  return typeof value === 'string' && value ? value : `引用文档 ${index + 1}`;
}

export function AgentRunPanel({ state }: { state: AgentRunViewState }) {
  const steps = state.plan?.steps ?? state.rounds.map((round) => ({ step_no: round.round_no, title: round.action_summary || `执行轮次 ${round.round_no}`, status: round.status }));
  return (
    <Stack spacing={1.4} p={1.7} sx={{ overflow: 'auto', height: '100%' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Typography sx={{ color: '#182441', fontSize: 18, fontWeight: 700 }}>Agent 运行</Typography>
        <Chip size="small" icon={state.status === 'running' ? <SyncOutlined /> : <CheckCircleOutlined />} label={statusLabel[state.status]} sx={{ bgcolor: state.status === 'running' ? '#e9f8ee' : state.status === 'completed' ? '#e9f8ee' : '#f1f3f7', color: state.status === 'running' || state.status === 'completed' ? '#2b9a52' : '#778297', fontWeight: 600 }} />
      </Stack>
      {state.router && (
        <Alert severity="info" sx={{ py: 0.5 }}><strong>{state.router.execution_mode === 'react' ? 'ReAct 协作模式' : '计划执行模式'}</strong><br />{state.router.reason_summary}</Alert>
      )}
      {state.error && <Alert severity="error">{state.error.message}</Alert>}
      <Paper variant="outlined" sx={{ p: 1.3, borderRadius: 2.5, borderColor: '#e2e6ee' }}>
        <Typography sx={{ color: '#26324d', fontSize: 13, fontWeight: 700, mb: 1.2 }}>执行步骤</Typography>
        {steps.length > 0 ? (
          <Stack spacing={0.6}>
            {steps.map((step, index) => {
              const completed = step.status === 'completed' || step.status === 'succeeded';
              const running = step.status === 'running';
              return (
                <Stack key={step.step_no} direction="row" spacing={1} alignItems="center" sx={{ minHeight: 47, px: 1, borderRadius: 1.7, bgcolor: running ? '#f0f2ff' : 'transparent', position: 'relative' }}>
                  <Box sx={{ width: 22, height: 22, borderRadius: '50%', display: 'grid', placeItems: 'center', flexShrink: 0, bgcolor: completed ? '#29aa55' : running ? '#4e64ed' : '#e0e4ec', color: '#fff', fontSize: 11 }}>{completed ? <CheckCircleOutlined sx={{ fontSize: 15 }} /> : index + 1}</Box>
                  <Box minWidth={0} flexGrow={1}><Typography noWrap sx={{ color: '#36425b', fontSize: 12.5 }}>{step.title}</Typography><Typography sx={{ color: running ? '#4e64ed' : '#8994a7', fontSize: 10.5 }}>{completed ? '已完成' : running ? '运行中' : '等待中'}</Typography></Box>
                </Stack>
              );
            })}
          </Stack>
        ) : <Typography sx={{ color: '#8792a6', fontSize: 12, py: 2 }}>提问后将在这里显示 Agent 执行步骤。</Typography>}
      </Paper>

      <Paper variant="outlined" sx={{ p: 1.3, borderRadius: 2.5, borderColor: '#e2e6ee' }}>
        <Typography sx={{ color: '#26324d', fontSize: 13, fontWeight: 700, mb: 1 }}>本次使用工具</Typography>
        {state.tools.length > 0 ? <Stack spacing={0.7}>{state.tools.map((tool) => (
          <Stack key={tool.tool_call_id} direction="row" alignItems="center" spacing={1} sx={{ minHeight: 42, px: 1, border: '1px solid #e7eaf0', borderRadius: 1.6 }}>
            <BuildOutlined sx={{ color: '#5370ee', fontSize: 18 }} />
            <Typography noWrap sx={{ color: '#56627a', fontSize: 11.5, flexGrow: 1 }}>{tool.tool_name}</Typography>
            <Typography sx={{ color: tool.status === 'completed' ? '#2ca457' : '#5670ec', fontSize: 10.5 }}>{tool.status === 'completed' ? '✓ 已完成' : '◌ 运行中'}</Typography>
          </Stack>
        ))}</Stack> : <Typography sx={{ color: '#8792a6', fontSize: 12 }}>暂无工具调用</Typography>}
      </Paper>

      <Paper variant="outlined" sx={{ p: 1.3, borderRadius: 2.5, borderColor: '#e2e6ee' }}>
        <Typography sx={{ color: '#26324d', fontSize: 13, fontWeight: 700, mb: 1 }}>引用文档</Typography>
        {state.citations.length > 0 ? <Stack spacing={0.7}>{state.citations.slice(0, 4).map((citation, index) => (
          <Stack key={`${citationTitle(citation, index)}-${index}`} direction="row" spacing={1} alignItems="center" sx={{ minHeight: 44, px: 1, borderRadius: 1.5, bgcolor: '#f7f8fb' }}>
            <Box sx={{ width: 28, height: 28, borderRadius: 1.2, display: 'grid', placeItems: 'center', bgcolor: '#e9edff', color: '#5068e9' }}><DescriptionOutlined sx={{ fontSize: 17 }} /></Box>
            <Typography noWrap title={citationTitle(citation, index)} sx={{ color: '#56627a', fontSize: 11.5, flexGrow: 1 }}>{citationTitle(citation, index)}</Typography>
          </Stack>
        ))}<Button size="small">查看全部（{state.citations.length}）</Button></Stack> : <Typography sx={{ color: '#8792a6', fontSize: 12 }}>暂无引用文档</Typography>}
      </Paper>
      {state.usage && <><Divider /><Stack direction="row" alignItems="center" spacing={1}><SmartToyOutlined sx={{ color: '#6574ec', fontSize: 18 }} /><Typography sx={{ color: '#7b879b', fontSize: 11 }}>本次使用 {state.usage.total_tokens} Tokens</Typography></Stack></>}
    </Stack>
  );
}
