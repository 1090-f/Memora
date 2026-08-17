import ArrowForwardOutlined from '@mui/icons-material/ArrowForwardOutlined';
import ContentCopyOutlined from '@mui/icons-material/ContentCopyOutlined';
import SmartToyOutlined from '@mui/icons-material/SmartToyOutlined';
import ThumbDownAltOutlined from '@mui/icons-material/ThumbDownAltOutlined';
import ThumbUpAltOutlined from '@mui/icons-material/ThumbUpAltOutlined';
import { Box, Button, IconButton, Paper, Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { InlineAgentRun } from '@/features/agent-run/components/InlineAgentRun';
import type { AgentRunViewState } from '@/features/agent-run/types';
import type { Message } from '../types';

const suggestions = ['总结当前知识库', '列出关键概念', '生成学习路径'];

export function MessageList({ messages, streamingAnswer, agentRunState, agentRunId, emptyComposer, onSuggestion }: {
  messages: Message[];
  streamingAnswer: string;
  agentRunState: AgentRunViewState;
  agentRunId: string | null;
  emptyComposer?: ReactNode;
  onSuggestion: (suggestion: string) => void;
}) {
  if (messages.length === 0 && !streamingAnswer) {
    return (
      <Stack alignItems="center" justifyContent="center" spacing={1.4} sx={{ flex: 1, width: '100%', px: 2.5, pb: { xs: 3, md: 10 } }}>
        <Typography sx={{ color: '#172343', fontSize: { xs: 24, md: 28 }, fontWeight: 650 }}>👋 你好！有什么想聊的吗？</Typography>
        <Typography sx={{ color: '#8791a2', fontSize: 13.5 }}>我会结合当前知识库回答，并在对话中展示 Agent 的工作过程。</Typography>
        {emptyComposer && <Box sx={{ width: 'min(900px, 100%)', pt: 2.3 }}>{emptyComposer}</Box>}
        <Stack direction="row" useFlexGap spacing={1} flexWrap="wrap" justifyContent="center" sx={{ pt: 0.5 }}>
          {suggestions.map((suggestion) => <Button key={suggestion} variant="outlined" endIcon={<ArrowForwardOutlined />} onClick={() => onSuggestion(suggestion)} sx={{ borderRadius: 2 }}>{suggestion}</Button>)}
        </Stack>
      </Stack>
    );
  }
  return (
    <Stack spacing={2.2} alignItems="center" sx={{ flex: 1, overflow: 'auto', px: { xs: 2, md: 4 }, py: 3, bgcolor: '#fff' }}>
      <Stack spacing={2.2} sx={{ width: '100%', maxWidth: 980, flexShrink: 0 }}>
      {messages.map((message) => (
        <Box key={message.id}>
        {message.role === 'assistant' && message.agent_run_id === agentRunId && <InlineAgentRun state={agentRunState} />}
        <Stack direction={message.role === 'user' ? 'row-reverse' : 'row'} spacing={1.2} alignItems="flex-start" sx={{ mt: message.role === 'assistant' && message.agent_run_id === agentRunId ? 1.2 : 0, width: message.role === 'assistant' ? '100%' : 'fit-content', ml: message.role === 'user' ? 'auto' : 0, maxWidth: message.role === 'user' ? '78%' : '100%' }}>
          <Box sx={{ width: 34, height: 34, flexShrink: 0, borderRadius: message.role === 'user' ? '50%' : 2, display: 'grid', placeItems: 'center', color: '#fff', background: message.role === 'user' ? 'linear-gradient(145deg,#5683f7,#5862e9)' : 'linear-gradient(145deg,#697af6,#704de5)', boxShadow: '0 7px 16px rgba(75,74,220,.2)', fontSize: 12 }}>
            {message.role === 'user' ? 'A' : <SmartToyOutlined sx={{ fontSize: 20 }} />}
          </Box>
          <Box minWidth={0} sx={{ flex: message.role === 'assistant' ? 1 : 'initial' }}>
            <Paper variant="outlined" sx={{ p: message.role === 'user' ? '13px 16px' : 2, borderRadius: message.role === 'user' ? '14px 4px 14px 14px' : '4px 14px 14px 14px', borderColor: message.role === 'user' ? 'transparent' : '#e1e5ed', bgcolor: message.role === 'user' ? '#efefff' : '#fff', boxShadow: message.role === 'user' ? 'none' : '0 5px 18px rgba(31,45,90,.035)' }}>
              <Typography sx={{ color: '#25314c', fontSize: 14, lineHeight: 1.75, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{message.content}</Typography>
              {message.citations && message.citations.length > 0 && (
                <Stack direction="row" useFlexGap flexWrap="wrap" spacing={0.8} sx={{ mt: 1.2 }}>
                  {message.citations.slice(0, 3).map((citation, index) => <Button key={`${message.id}-${index}`} size="small" variant="outlined" sx={{ fontSize: 11, borderRadius: 1.5 }}>{citation.document_title || citation.title || `引用 ${index + 1}`}</Button>)}
                </Stack>
              )}
              {message.role === 'assistant' && (
                <Stack direction="row" justifyContent="flex-end" sx={{ mt: 0.7, mb: -0.8, mr: -0.8 }}>
                  <IconButton size="small" aria-label="复制回答" onClick={() => void navigator.clipboard?.writeText(message.content)}><ContentCopyOutlined sx={{ fontSize: 16 }} /></IconButton>
                  <IconButton size="small" aria-label="回答有帮助"><ThumbUpAltOutlined sx={{ fontSize: 16 }} /></IconButton>
                  <IconButton size="small" aria-label="回答无帮助"><ThumbDownAltOutlined sx={{ fontSize: 16 }} /></IconButton>
                </Stack>
              )}
            </Paper>
            <Typography sx={{ color: '#8c97aa', fontSize: 10.5, mt: 0.5, textAlign: message.role === 'user' ? 'right' : 'left' }}>{new Date(message.created_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</Typography>
          </Box>
        </Stack></Box>
      ))}
      {agentRunState.status !== 'idle' && !messages.some((message) => message.role === 'assistant' && message.agent_run_id === agentRunId) && (
        <InlineAgentRun state={agentRunState} />
      )}
      {streamingAnswer && (
        <Stack direction="row" spacing={1.2} alignItems="flex-start" sx={{ width: '100%' }}>
          <Box sx={{ width: 34, height: 34, flexShrink: 0, borderRadius: 2, display: 'grid', placeItems: 'center', color: '#fff', background: 'linear-gradient(145deg,#697af6,#704de5)' }}><SmartToyOutlined sx={{ fontSize: 20 }} /></Box>
          <Paper variant="outlined" sx={{ flex: 1, minWidth: 0, p: 2, borderRadius: '4px 14px 14px 14px', borderColor: '#e1e5ed' }}><Typography sx={{ color: '#25314c', fontSize: 14, lineHeight: 1.75, whiteSpace: 'pre-wrap' }}>{streamingAnswer}</Typography></Paper>
        </Stack>
      )}
      </Stack>
    </Stack>
  );
}
