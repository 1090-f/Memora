import { Box } from '@mui/material';
import type { ReactNode } from 'react';

export function ChatWorkspace({ messages, composer }: {
  messages: ReactNode;
  composer: ReactNode;
}) {
  return (
    <Box component="main" aria-label="对话区" sx={{ display: 'flex', flexDirection: 'column', width: '100%', height: '100dvh', minHeight: 0, overflow: 'hidden', bgcolor: '#fff' }}>
      {messages}{composer}
    </Box>
  );
}
