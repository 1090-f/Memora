import { Box, Button, Paper, Stack, Typography } from '@mui/material';
import type { Message } from '../types';

const suggestions = ['总结当前知识库', '列出关键概念', '生成学习路径'];

export function MessageList({ messages, streamingAnswer, onSuggestion }: {
  messages: Message[];
  streamingAnswer: string;
  onSuggestion: (suggestion: string) => void;
}) {
  if (messages.length === 0 && !streamingAnswer) {
    return (
      <Stack alignItems="center" justifyContent="center" spacing={2} sx={{ flex: 1, p: 4 }}>
        <Typography variant="h5" fontWeight={750}>等待提问</Typography>
        <Typography color="text.secondary">选择一个建议或输入自己的问题。</Typography>
        <Stack direction="row" spacing={1} flexWrap="wrap" justifyContent="center">
          {suggestions.map((suggestion) => <Button key={suggestion} variant="outlined" onClick={() => onSuggestion(suggestion)}>{suggestion}</Button>)}
        </Stack>
      </Stack>
    );
  }
  return (
    <Stack spacing={2} sx={{ flex: 1, overflow: 'auto', p: 3 }}>
      {messages.map((message) => {
        const failed = message.role === 'assistant' && message.status === 'failed';
        return (
          <Box key={message.id} alignSelf={message.role === 'user' ? 'flex-end' : 'stretch'} maxWidth="82%">
            <Paper
              variant="outlined"
              sx={{
                p: 2,
                bgcolor: message.role === 'user' ? '#efefff' : failed ? '#fff4f2' : '#fff',
                borderColor: failed ? 'error.light' : undefined,
              }}
            >
              {failed && <Typography color="error" variant="caption" sx={{ display: 'block', mb: 0.5 }}>运行失败</Typography>}
              <Typography sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{message.content}</Typography>
            </Paper>
          </Box>
        );
      })}
      {streamingAnswer && <Paper variant="outlined" sx={{ p: 2 }}><Typography sx={{ whiteSpace: 'pre-wrap' }}>{streamingAnswer}</Typography></Paper>}
    </Stack>
  );
}
