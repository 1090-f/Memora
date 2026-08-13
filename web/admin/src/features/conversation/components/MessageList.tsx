import { ContentCopyOutlined, ReplayOutlined } from '@mui/icons-material';
import { Box, Button, IconButton, Paper, Stack, Tooltip, Typography } from '@mui/material';
import { useEffect, useRef, useState } from 'react';
import { MarkdownViewer } from '@/features/document/components/preview/MarkdownViewer';
import type { Message } from '../types';

const suggestions = ['总结当前知识库', '列出关键概念', '生成学习路径'];

/**
 * MessageList 渲染会话消息列表，支持消息级操作（复制、重试）。
 * - 左侧消息（助手）的操作按钮位于右下角
 * - 右侧消息（用户）的操作按钮位于左下角
 * - 复制按钮：点击后复制消息文本到剪贴板
 * - 重试按钮：仅助手消息显示，基于 agent_run_id 调用重试 API
 */
export function MessageList({ messages, streamingAnswer, onSuggestion, scrollToBottom, onRetry }: {
  messages: Message[];
  streamingAnswer: string;
  onSuggestion: (suggestion: string) => void;
  scrollToBottom?: boolean;
  onRetry?: (agentRunId: string, query: string) => void;
}) {
  const bottomRef = useRef<HTMLDivElement>(null);

  // 当 scrollToBottom 为 true 或有新消息时，自动滚动到底部
  useEffect(() => {
    if (scrollToBottom && bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [scrollToBottom, messages.length, streamingAnswer]);

  if (messages.length === 0 && !streamingAnswer) {
    return (
      <Stack alignItems="center" justifyContent="center" spacing={2} sx={{ flex: 1, p: 4 }}>
        <Typography variant="h5" fontWeight={750}>等待提问</Typography>
        <Typography color="text.secondary">选择一个建议或输入自己的问题。</Typography>
        <Stack direction="row" spacing={1} flexWrap="wrap" justifyContent="center">
          {suggestions.map((suggestion) => (
            <Button key={suggestion} variant="outlined" onClick={() => onSuggestion(suggestion)}>
              {suggestion}
            </Button>
          ))}
        </Stack>
      </Stack>
    );
  }

  return (
    <Stack spacing={2} sx={{ flex: 1, overflow: 'auto', p: 3 }}>
      {messages.map((message) => {
        const failed = message.role === 'assistant' && message.status === 'failed';
        const isUser = message.role === 'user';
        return (
          <Box key={message.id} alignSelf={isUser ? 'flex-end' : 'stretch'} maxWidth="82%">
            {/* 消息气泡 */}
            <Paper
              variant="outlined"
              sx={{
                p: 2,
                bgcolor: isUser ? '#efefff' : failed ? '#fff4f2' : '#fff',
                borderColor: failed ? 'error.light' : undefined,
              }}
            >
              {/* 失败提示 */}
              {failed && <Typography color="error" variant="caption" sx={{ display: 'block', mb: 0.5 }}>运行失败</Typography>}

              {/* 消息正文 */}
              {isUser
                ? <Typography sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{message.content}</Typography>
                : <MarkdownViewer content={message.content} />}
            </Paper>

            {/* 操作按钮 — 在气泡外面紧挨着，用户消息在左下，助手消息在右下 */}
            <MessageActions
              isUser={isUser}
              content={message.content}
              agentRunId={message.agent_run_id}
              onRetry={onRetry}
            />
          </Box>
        );
      })}

      {/* 流式回答占位 */}
      {streamingAnswer && (
        <Paper variant="outlined" sx={{ p: 2 }}>
          <MarkdownViewer content={streamingAnswer} />
        </Paper>
      )}

      {/* 滚动锚点 */}
      <div ref={bottomRef} />
    </Stack>
  );
}

/**
 * MessageActions 渲染消息底部操作按钮（复制、重试）。
 * 用户消息：按钮在左下角；助手消息：按钮在右下角。
 */
function MessageActions({ isUser, content, agentRunId, onRetry }: {
  isUser: boolean;
  content: string;
  agentRunId: string | null;
  onRetry?: (agentRunId: string, query: string) => void;
}) {
  const [copied, setCopied] = useState(false);

  // 复制按钮点击处理：将文本写入剪贴板并短暂显示"已复制"状态
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(content);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000); // 2秒后恢复
    } catch {
      // 剪贴板写入失败时静默忽略
    }
  };

  // 重试按钮是否显示：仅助手消息且有 agent_run_id 时才显示
  const showRetry = !isUser && agentRunId && onRetry;

  return (
    <Box
      sx={{
        display: 'flex',
        gap: 0.25,
        mt: 0.75,
        justifyContent: isUser ? 'flex-start' : 'flex-end',
        opacity: 0.5,
        transition: 'opacity 0.2s',
        '&:hover': { opacity: 1 },
      }}
    >
      {/* 复制按钮 */}
      <Tooltip title={copied ? '已复制' : '复制内容'} placement={isUser ? 'bottom' : 'bottom-end'}>
        <IconButton size="small" onClick={handleCopy} sx={{ width: 28, height: 28 }}>
          {copied ? (
            <Typography variant="caption" sx={{ fontSize: '0.65rem', fontWeight: 600, color: 'success.main' }}>
              已复制
            </Typography>
          ) : (
            <ContentCopyOutlined sx={{ fontSize: '0.9rem' }} />
          )}
        </IconButton>
      </Tooltip>

      {/* 重试按钮（仅助手消息） */}
      {showRetry && (
        <Tooltip title="重新生成" placement="bottom">
          <IconButton
            size="small"
            onClick={() => onRetry(agentRunId, content)}
            sx={{ width: 28, height: 28 }}
          >
            <ReplayOutlined sx={{ fontSize: '0.9rem' }} />
          </IconButton>
        </Tooltip>
      )}
    </Box>
  );
}