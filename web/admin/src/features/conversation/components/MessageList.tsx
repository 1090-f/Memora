import ArrowForwardOutlined from '@mui/icons-material/ArrowForwardOutlined';
import ContentCopyOutlined from '@mui/icons-material/ContentCopyOutlined';
import CheckOutlined from '@mui/icons-material/CheckOutlined';
import ReplayOutlined from '@mui/icons-material/ReplayOutlined';
import SmartToyOutlined from '@mui/icons-material/SmartToyOutlined';
import { Box, Button, IconButton, Paper, Stack, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useState, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { InlineAgentRun } from '@/features/agent-run/components/InlineAgentRun';
import type { AgentRunViewState } from '@/features/agent-run/types';
import type { Message } from '../types';

const suggestions = ['总结当前知识库', '列出关键概念', '生成学习路径'];

const markdownSx = {
  overflowWrap: 'anywhere',
  '& > :first-of-type': { mt: 0 },
  '& > :last-child': { mb: 0 },
  '& h1, & h2, & h3, & h4, & h5, & h6': { mt: 2.5, mb: 1, lineHeight: 1.35 },
  '& p, & ul, & ol, & blockquote': { my: 1.25, lineHeight: 1.75 },
  '& pre': { overflowX: 'auto', p: 1.5, borderRadius: 1, bgcolor: 'action.hover' },
  '& code': { fontFamily: 'Consolas, "SFMono-Regular", monospace' },
  '& :not(pre) > code': { px: 0.5, py: 0.2, borderRadius: 0.5, bgcolor: 'action.hover' },
  '& table': { display: 'block', maxWidth: '100%', overflowX: 'auto', borderCollapse: 'collapse', my: 2 },
  '& th, & td': { border: 1, borderColor: 'divider', px: 1.25, py: 0.75, textAlign: 'left' },
  '& blockquote': { ml: 0, pl: 2, borderLeft: 4, borderColor: 'divider', color: 'text.secondary' },
  '& img': { maxWidth: '100%', maxHeight: 480, width: 'auto', height: 'auto', cursor: 'zoom-in' },
} as const;

function MarkdownContent({ content }: { content: string }) {
  if (!content) {
    return (
      <Typography sx={{ color: '#8c97aa', fontSize: 13, fontStyle: 'italic' }}>
        （空回复）
      </Typography>
    );
  }
  return (
    <Box sx={markdownSx}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          img: ({ node, ...props }) => {
            void node;
            return <img {...props} onClick={() => props.src && !props.src.startsWith('data:') && window.open(props.src, '_blank', 'noopener')} />;
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </Box>
  );
}

/**
 * Determine the effective content of a message considering version switching.
 */
function getEffectiveContent(message: Message): string {
  if (message.versions && message.versions.length > 0 && message.current_version_index !== undefined && message.current_version_index >= 0) {
    const version = message.versions[message.current_version_index];
    if (version) return version.content;
  }
  return message.content;
}

/**
 * Determine the effective agent_run_id considering version switching.
 * When viewing a historical version, returns that version's agent_run_id.
 */
function getEffectiveAgentRunId(message: Message): string | null {
  if (message.versions && message.versions.length > 0 && message.current_version_index !== undefined && message.current_version_index >= 0) {
    const version = message.versions[message.current_version_index];
    if (version && version.agent_run_id) return version.agent_run_id;
  }
  return message.agent_run_id;
}

export function MessageList({ messages, streamingAnswer, agentRunState, agentRunId, agentRunStates, retryingMessageId, resumingRun, emptyComposer, onSuggestion, onRetry, onSwitchVersion }: {
  messages: Message[];
  streamingAnswer: string;
  agentRunState: AgentRunViewState;
  agentRunId: string | null;
  agentRunStates?: Record<string, AgentRunViewState>;
  retryingMessageId: string | null;
  resumingRun: boolean;
  emptyComposer?: ReactNode;
  onSuggestion: (suggestion: string) => void;
  onRetry?: (agentRunId: string) => void;
  onSwitchVersion?: (messageId: string, versionIdx: number) => void;
}) {
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const handleCopy = useCallback(async (text: string, messageId: string) => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }
      setCopiedId(messageId);
      setTimeout(() => setCopiedId((prev) => prev === messageId ? null : prev), 2000);
    } catch {
      // ignore clipboard errors
    }
  }, []);

  // Find the last assistant message with an agent_run_id and the latest user question.
  const lastAssistantMsg = [...messages].reverse().find((m) => m.role === 'assistant' && m.agent_run_id);
  const lastUserMessage = [...messages].reverse().find((m) => m.role === 'user');
  const shouldShowRunUnderUser = agentRunState.status !== 'idle'
    && !retryingMessageId
    && !messages.some((message) => message.role === 'assistant' && message.agent_run_id === agentRunId);
  const hidePreviousAssistantReply = shouldShowRunUnderUser && resumingRun && Boolean(lastAssistantMsg)
    && lastAssistantMsg?.agent_run_id === agentRunId;

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
        {/* Historical agent run process — shown above completed assistant messages */}
        {(() => {
          const effectiveRunId = getEffectiveAgentRunId(message);
          const runState = effectiveRunId ? agentRunStates?.[effectiveRunId] : undefined;
          return message.role === 'assistant' && effectiveRunId && message.id !== retryingMessageId
            && runState && runState.status !== 'idle' && (
            <Box sx={{ mb: 1.5 }}>
              <InlineAgentRun state={runState} runId={effectiveRunId ?? undefined} />
            </Box>
          );
        })()}
        {message.role === 'assistant' && message.id === retryingMessageId && <InlineAgentRun state={agentRunState} runId={agentRunId ?? undefined} />}
        {message.id !== retryingMessageId && !(hidePreviousAssistantReply && message.id === lastAssistantMsg?.id) && <Stack direction={message.role === 'user' ? 'row-reverse' : 'row'} spacing={1.2} alignItems="flex-start" sx={{ mt: message.role === 'assistant' && message.id === retryingMessageId ? 1.2 : 0, width: message.role === 'assistant' ? '100%' : 'fit-content', ml: message.role === 'user' ? 'auto' : 0, maxWidth: message.role === 'user' ? '78%' : '100%' }}> 
          <Box sx={{ width: 34, height: 34, flexShrink: 0, borderRadius: message.role === 'user' ? '50%' : 2, display: 'grid', placeItems: 'center', color: '#fff', background: message.role === 'user' ? 'linear-gradient(145deg,#5683f7,#5862e9)' : 'linear-gradient(145deg,#697af6,#704de5)', boxShadow: '0 7px 16px rgba(75,74,220,.2)', fontSize: 12 }}>
            {message.role === 'user' ? 'A' : <SmartToyOutlined sx={{ fontSize: 20 }} />}
          </Box>
          <Box minWidth={0} sx={{ flex: message.role === 'assistant' ? 1 : 'initial' }}>
            <Paper variant="outlined" sx={{ p: message.role === 'user' ? '13px 16px' : 2, borderRadius: message.role === 'user' ? '14px 4px 14px 14px' : '4px 14px 14px 14px', borderColor: message.role === 'user' ? 'transparent' : '#e1e5ed', bgcolor: message.role === 'user' ? '#efefff' : '#fff', boxShadow: message.role === 'user' ? 'none' : '0 5px 18px rgba(31,45,90,.035)' }}>
              <Box sx={{ color: '#25314c', fontSize: 14, lineHeight: 1.75 }}>
                <MarkdownContent content={getEffectiveContent(message)} />
              </Box>
              {message.citations && message.citations.length > 0 && (
                <Stack direction="row" useFlexGap flexWrap="wrap" spacing={0.8} sx={{ mt: 1.2 }}>
                  {message.citations.slice(0, 3).map((citation, index) => <Button key={`${message.id}-${index}`} size="small" variant="outlined" sx={{ fontSize: 11, borderRadius: 1.5 }}>{citation.document_title || citation.title || `引用 ${index + 1}`}</Button>)}
                </Stack>
              )}
              {/* Version switcher — only for assistant messages with versions */}
              {message.role === 'assistant' && message.versions && message.versions.length > 0 && (
                <Stack direction="row" alignItems="center" spacing={0.8} sx={{ mt: 1, mb: 0.5 }}>
                  <Typography sx={{ color: '#8c97aa', fontSize: 11 }}>版本：</Typography>
                  <ToggleButtonGroup
                    size="small"
                    value={message.current_version_index ?? -1}
                    exclusive
                    onChange={(_, newValue) => {
                      if (newValue !== null && onSwitchVersion) {
                        onSwitchVersion(message.id, newValue);
                      }
                    }}
                    sx={{
                      '& .MuiToggleButton-root': {
                        fontSize: 11,
                        px: 1.2,
                        py: 0.2,
                        borderRadius: 1,
                        border: '1px solid',
                        borderColor: '#e1e5ed',
                        color: '#6d788a',
                        '&.Mui-selected': {
                          bgcolor: '#eef2ff',
                          color: '#4f63e5',
                          borderColor: '#c5d0ff',
                        },
                      },
                    }}
                  >
                    <ToggleButton value={-1}>最新</ToggleButton>
                    {message.versions.map((_, vi) => (
                      <ToggleButton key={vi} value={vi}>V{vi + 1}</ToggleButton>
                    ))}
                  </ToggleButtonGroup>
                </Stack>
              )}
              {/* Bottom action bar for both user and assistant messages */}
              <Stack direction="row" justifyContent={message.role === 'user' ? 'flex-start' : 'space-between'} alignItems="center" sx={{ mt: 0.7, mb: -0.8, mr: -0.8 }}>
                <Stack direction="row" spacing={0.2}>
                  <IconButton size="small" aria-label="复制" onClick={() => void handleCopy(getEffectiveContent(message), message.id)}>
                    {copiedId === message.id ? <CheckOutlined sx={{ fontSize: 16, color: '#4caf50' }} /> : <ContentCopyOutlined sx={{ fontSize: 16 }} />}
                  </IconButton>
                  {message.role === 'assistant' && message.agent_run_id && message.id === lastAssistantMsg?.id && onRetry && (
                    <IconButton size="small" aria-label="重试" onClick={() => onRetry(message.agent_run_id!)}>
                      <ReplayOutlined sx={{ fontSize: 16 }} />
                    </IconButton>
                  )}
                </Stack>
              </Stack>
            </Paper>
            <Typography sx={{ color: '#8c97aa', fontSize: 10.5, mt: 0.5, textAlign: message.role === 'user' ? 'right' : 'left' }}>{new Date(message.created_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}</Typography>
          </Box>
        </Stack>}
        {message.id === lastUserMessage?.id && shouldShowRunUnderUser && <InlineAgentRun state={agentRunState} runId={agentRunId ?? undefined} />}
        </Box>
       ))}
      {streamingAnswer && (
        <Stack direction="row" spacing={1.2} alignItems="flex-start" sx={{ width: '100%' }}>
          <Box sx={{ width: 34, height: 34, flexShrink: 0, borderRadius: 2, display: 'grid', placeItems: 'center', color: '#fff', background: 'linear-gradient(145deg,#697af6,#704de5)' }}><SmartToyOutlined sx={{ fontSize: 20 }} /></Box>
          <Paper variant="outlined" sx={{ flex: 1, minWidth: 0, p: 2, borderRadius: '4px 14px 14px 14px', borderColor: '#e1e5ed' }}>
            <MarkdownContent content={streamingAnswer} />
          </Paper>
        </Stack>
      )}
      </Stack>
    </Stack>
  );
}