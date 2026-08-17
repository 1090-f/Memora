import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import BuildOutlined from '@mui/icons-material/BuildOutlined';
import CheckCircleRounded from '@mui/icons-material/CheckCircleRounded';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import ErrorOutlineRounded from '@mui/icons-material/ErrorOutlineRounded';
import ExpandMoreRounded from '@mui/icons-material/ExpandMoreRounded';
import SyncRounded from '@mui/icons-material/SyncRounded';
import { Box, Chip, Collapse, IconButton, Stack, Typography } from '@mui/material';
import { useState } from 'react';
import type { AgentRunViewState } from '../types';

const statusText = {
  idle: '等待中',
  queued: '正在排队',
  running: '正在运行',
  completed: '运行完成',
  failed: '运行失败',
  cancelled: '已停止',
} as const;

function citationName(citation: Record<string, unknown>, index: number) {
  const value = citation.document_title || citation.title || citation.url;
  return typeof value === 'string' && value ? value : `引用 ${index + 1}`;
}

export function InlineAgentRun({ state }: { state: AgentRunViewState }) {
  const [expanded, setExpanded] = useState(state.status === 'running' || state.status === 'queued');
  const running = state.status === 'running' || state.status === 'queued';
  const failed = state.status === 'failed';
  const steps = state.plan?.steps ?? state.rounds.map((round) => ({
    step_no: round.round_no,
    title: round.action_summary || `执行轮次 ${round.round_no}`,
    status: round.status,
  }));
  const detailCount = steps.length + state.tools.length + state.citations.length;

  if (state.status === 'idle') return null;

  return (
    <Box
      component="section"
      aria-label="Agent 运行过程"
      sx={{
        width: 'calc(100% - 46px)',
        ml: '46px',
        border: '1px solid',
        borderColor: failed ? '#f1c8c8' : '#e0e6ef',
        borderRadius: 2.5,
        bgcolor: '#f8fafc',
        overflow: 'hidden',
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        spacing={1.1}
        onClick={() => setExpanded((value) => !value)}
        sx={{ px: 1.5, py: 1.2, cursor: 'pointer', userSelect: 'none' }}
      >
        <Box sx={{ width: 28, height: 28, display: 'grid', placeItems: 'center', borderRadius: 1.4, color: '#5268e8', bgcolor: '#e9edff' }}>
          <AutoAwesomeOutlined sx={{ fontSize: 17 }} />
        </Box>
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Stack direction="row" alignItems="center" spacing={0.8}>
            <Typography sx={{ color: '#26324d', fontSize: 13, fontWeight: 700 }}>Agent 运行</Typography>
            <Chip
              size="small"
              icon={running ? <SyncRounded className="animate-spin" /> : failed ? <ErrorOutlineRounded /> : <CheckCircleRounded />}
              label={statusText[state.status]}
              sx={{
                height: 22,
                bgcolor: failed ? '#fff0f0' : running ? '#edf1ff' : '#eaf8ef',
                color: failed ? '#bd4a4a' : running ? '#4e63dd' : '#278b4d',
                fontSize: 10.5,
                fontWeight: 650,
                '& .MuiChip-icon': { color: 'inherit', fontSize: 14 },
              }}
            />
          </Stack>
          <Typography noWrap sx={{ color: '#7b8799', fontSize: 11.5, mt: 0.15 }}>
            {state.error?.message || state.router?.reason_summary || (running ? '正在分析问题并调用所需工具' : `${detailCount} 项运行记录`)}
          </Typography>
        </Box>
        <IconButton size="small" aria-label={expanded ? '收起运行详情' : '展开运行详情'} sx={{ color: '#7c8799' }}>
          <ExpandMoreRounded sx={{ fontSize: 20, transform: expanded ? 'rotate(180deg)' : 'none', transition: 'transform .2s' }} />
        </IconButton>
      </Stack>

      <Collapse in={expanded}>
        <Stack spacing={1.3} sx={{ borderTop: '1px solid #e5e9f0', px: 1.6, py: 1.4 }}>
          {steps.length > 0 && (
            <Stack spacing={0.8}>
              <Typography sx={{ color: '#6d788a', fontSize: 11, fontWeight: 700 }}>执行步骤</Typography>
              {steps.map((step, index) => {
                const completed = step.status === 'completed' || step.status === 'succeeded';
                const active = step.status === 'running';
                return (
                  <Stack key={`${step.step_no}-${index}`} direction="row" spacing={1} alignItems="center">
                    <Box sx={{ width: 18, height: 18, flexShrink: 0, display: 'grid', placeItems: 'center', borderRadius: '50%', bgcolor: completed ? '#31a75b' : active ? '#586de8' : '#dce1e9', color: '#fff', fontSize: 10 }}>
                      {completed ? <CheckCircleRounded sx={{ fontSize: 14 }} /> : index + 1}
                    </Box>
                    <Typography sx={{ color: active ? '#4358d0' : '#4e5a6f', fontSize: 12.5, flex: 1 }}>{step.title}</Typography>
                    <Typography sx={{ color: '#8c96a7', fontSize: 10.5 }}>{completed ? '已完成' : active ? '进行中' : '等待中'}</Typography>
                  </Stack>
                );
              })}
            </Stack>
          )}

          {state.tools.length > 0 && (
            <Stack spacing={0.7}>
              <Typography sx={{ color: '#6d788a', fontSize: 11, fontWeight: 700 }}>工具调用</Typography>
              <Stack direction="row" useFlexGap flexWrap="wrap" gap={0.7}>
                {state.tools.map((tool) => (
                  <Chip key={tool.tool_call_id} size="small" icon={<BuildOutlined />} label={tool.tool_name} variant="outlined" sx={{ bgcolor: '#fff', borderColor: '#dde3ec', color: '#536078', fontSize: 11, '& .MuiChip-icon': { color: '#5b6ee1' } }} />
                ))}
              </Stack>
            </Stack>
          )}

          {state.citations.length > 0 && (
            <Stack spacing={0.7}>
              <Typography sx={{ color: '#6d788a', fontSize: 11, fontWeight: 700 }}>引用文档</Typography>
              <Stack direction="row" useFlexGap flexWrap="wrap" gap={0.7}>
                {state.citations.slice(0, 5).map((citation, index) => (
                  <Chip key={`${citationName(citation, index)}-${index}`} size="small" icon={<DescriptionOutlined />} label={citationName(citation, index)} sx={{ maxWidth: 220, bgcolor: '#eef2fa', color: '#536078', fontSize: 11, '& .MuiChip-icon': { color: '#6574dc' } }} />
                ))}
              </Stack>
            </Stack>
          )}

          {state.usage && (
            <Typography sx={{ color: '#8a94a5', fontSize: 10.5 }}>
              输入 {state.usage.input_tokens} · 输出 {state.usage.output_tokens} · 共 {state.usage.total_tokens} Tokens
            </Typography>
          )}
        </Stack>
      </Collapse>
    </Box>
  );
}
