import { CheckOutlined, ChevronLeft, ChevronRight, ContentCopyOutlined, ReplayOutlined } from '@mui/icons-material';
import { Box, Button, IconButton, Paper, Stack, Tooltip, Typography } from '@mui/material';
import { useEffect, useRef, useState } from 'react';
import { MarkdownViewer } from '@/features/document/components/preview/MarkdownViewer';
import type { Message } from '../types';

const suggestions = ['总结当前知识库', '列出关键概念', '生成学习路径'];

/**
 * VersionSwitcher 版本切换器，类似 DeepSeek 的分页指示器。
 * versions 是历史版本数组（不含当前最新版本），total = versions.length + 1。
 * currentIndex: -1 表示显示最新版本（content）；>=0 表示显示对应历史版本。
 * onSwitch 回调的 newIndex 取值范围同 currentIndex。
 */
function VersionSwitcher({
  versions,
  currentIndex,
  onSwitch,
}: {
  versions: { content: string; agent_run_id: string; status?: string; created_at: string }[];
  currentIndex: number;
  onSwitch: (newIndex: number) => void;
}) {
  const total = versions.length + 1; // 历史版本数 + 当前最新版本
  // 显示用索引：-1 转为 total-1（最后一个 = 最新版本）
  const displayIndex = currentIndex === -1 ? total - 1 : currentIndex;

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.25 }}>
      <IconButton
        size="small"
        disabled={displayIndex <= 0}
        onClick={() => {
          // 向左翻：新的 displayIndex 减 1，若为 total-1 则映射为 -1
          const newDisplay = displayIndex - 1;
          onSwitch(newDisplay === total - 1 ? -1 : newDisplay);
        }}
        sx={{ width: 24, height: 24 }}
      >
        <ChevronLeft sx={{ fontSize: '1rem' }} />
      </IconButton>
      <Typography variant="caption" color="text.secondary" sx={{ fontSize: '0.75rem', minWidth: 32, textAlign: 'center' }}>
        {displayIndex + 1} / {total}
      </Typography>
      <IconButton
        size="small"
        disabled={displayIndex >= total - 1}
        onClick={() => {
          // 向右翻：新的 displayIndex 加 1，若为 total-1 则映射为 -1
          const newDisplay = displayIndex + 1;
          onSwitch(newDisplay === total - 1 ? -1 : newDisplay);
        }}
        sx={{ width: 24, height: 24 }}
      >
        <ChevronRight sx={{ fontSize: '1rem' }} />
      </IconButton>
    </Box>
  );
}

/**
 * 获取消息当前应展示的内容。
 * - current_version_index >= 0: 显示对应历史版本
 * - current_version_index === undefined 或 -1: 显示 message.content（最新版本）
 */
function getDisplayContent(message: Message): string {
  if (message.versions && message.versions.length > 0 && message.current_version_index !== undefined && message.current_version_index >= 0) {
    const v = message.versions[message.current_version_index];
    if (v) return v.content;
  }
  return message.content;
}

/**
 * 获取当前展示版本对应的 agent_run_id（用于重试）。
 */
function getDisplayAgentRunId(message: Message): string | null {
  if (message.versions && message.versions.length > 0 && message.current_version_index !== undefined && message.current_version_index >= 0) {
    const v = message.versions[message.current_version_index];
    if (v) return v.agent_run_id;
  }
  return message.agent_run_id;
}

/**
 * MessageList 渲染会话消息列表，支持消息级操作（复制、重试）和版本切换。
 */
export function MessageList({ messages, streamingAnswer, onSuggestion, scrollToBottom, onRetry, onSwitchVersion }: {
  messages: Message[];
  streamingAnswer: string;
  onSuggestion: (suggestion: string) => void;
  scrollToBottom?: boolean;
  onRetry?: (agentRunId: string, query: string) => void;
  onSwitchVersion?: (messageId: string, newIndex: number) => void;
}) {
  const bottomRef = useRef<HTMLDivElement>(null);

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

  // 找到最后一个有 agent_run_id 的助理消息（最新的 agent 运行消息）
  const latestAgentRunIndex = (() => {
    for (let i = messages.length - 1; i >= 0; i--) {
      const msg = messages[i];
      if (msg.role === 'assistant' && getDisplayAgentRunId(msg)) {
        return i;
      }
    }
    return -1;
  })();

  return (
    <Stack spacing={2} sx={{ flex: 1, overflow: 'auto', p: 3 }}>
      {messages.map((message, index) => {
        const failed = message.role === 'assistant' && message.status === 'failed';
        const isUser = message.role === 'user';
        const isLatestAgentRun = !isUser && index === latestAgentRunIndex;
        const displayContent = getDisplayContent(message);
        const displayAgentRunId = getDisplayAgentRunId(message);
        const hasVersions = !isUser && !!message.versions && message.versions.length > 0;
        const viewingHistory = hasVersions && message.current_version_index !== undefined && message.current_version_index >= 0;
        // totalVersions = 历史版本数 + 1（当前最新版本）
        const totalVersions = hasVersions ? (message.versions!.length + 1) : 1;

        return (
          <Box key={message.id} alignSelf={isUser ? 'flex-end' : 'stretch'} maxWidth="82%">
            {/* 消息气泡 */}
            <Paper
              variant="outlined"
              sx={{
                p: 2,
                bgcolor: isUser ? '#efefff' : failed ? '#fff4f2' : viewingHistory ? '#fafafa' : '#fff',
                borderColor: failed ? 'error.light' : viewingHistory ? 'divider' : undefined,
                opacity: viewingHistory ? 0.85 : 1,
              }}
            >
              {/* 失败提示 */}
              {failed && <Typography color="error" variant="caption" sx={{ display: 'block', mb: 0.5 }}>运行失败</Typography>}

              {/* 查看历史版本提示 */}
              {viewingHistory && (
                <Typography color="text.secondary" variant="caption" sx={{ display: 'block', mb: 0.5, fontStyle: 'italic' }}>
                  历史版本 {(message.current_version_index! >= 0 ? message.current_version_index! : 0) + 1} / {totalVersions}
                </Typography>
              )}

              {/* 消息正文 */}
              {isUser ? (
                <Typography sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{displayContent}</Typography>
              ) : displayContent ? (
                <MarkdownViewer content={displayContent} />
              ) : (
                /* 流式生成中或重试中，内容为空时显示占位 */
                <Typography color="text.secondary" fontStyle="italic" variant="body2">
                  {message.status === 'running' ? '正在重新生成...' : ''}
                </Typography>
              )}
            </Paper>

            {/* 操作栏：版本切换 + 复制/重试按钮，统一放在一行 */}
            <Box
              sx={{
                display: 'flex',
                gap: 0.5,
                mt: 0.75,
                alignItems: 'center',
                opacity: 0.5,
                transition: 'opacity 0.2s',
                '&:hover': { opacity: 1 },
              }}
            >
              {/* 左侧：版本切换（仅助手消息且有历史版本时显示） */}
              {!isUser && hasVersions && onSwitchVersion && (
                <VersionSwitcher
                  versions={message.versions!}
                  currentIndex={message.current_version_index ?? -1}
                  onSwitch={(newIndex) => onSwitchVersion(message.id, newIndex)}
                />
              )}

              {/* 弹性空间：将右侧按钮推到右侧 */}
              {!isUser && <Box sx={{ flex: 1 }} />}

              {/* 右侧：复制 + 重试（仅最新的 agent 运行消息显示重试按钮） */}
              <MessageActionButtons
                isUser={isUser}
                content={displayContent}
                agentRunId={displayAgentRunId}
                onRetry={isLatestAgentRun ? onRetry : undefined}
              />
            </Box>
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
 * MessageActionButtons 渲染复制和重试按钮。
 * 用户消息：按钮在左下角；助手消息：按钮在右下角（由父容器 flex 控制）。
 */
function MessageActionButtons({ isUser, content, agentRunId, onRetry }: {
  isUser: boolean;
  content: string;
  agentRunId: string | null;
  onRetry?: (agentRunId: string, query: string) => void;
}) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(content);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // 剪贴板写入失败时静默忽略
    }
  };

  const showRetry = !isUser && agentRunId && onRetry;

  return (
    <>
      <Tooltip title={copied ? '已复制' : '复制内容'} placement={isUser ? 'bottom' : 'bottom-end'}>
        <IconButton size="small" onClick={handleCopy} sx={{ width: 28, height: 28 }}>
          {copied ? (
            <CheckOutlined sx={{ fontSize: '0.9rem', color: 'success.main' }} />
          ) : (
            <ContentCopyOutlined sx={{ fontSize: '0.9rem' }} />
          )}
        </IconButton>
      </Tooltip>

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
    </>
  );
}