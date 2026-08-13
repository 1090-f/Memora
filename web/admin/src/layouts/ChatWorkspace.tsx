import { Box, Paper } from '@mui/material';
import type { ReactNode } from 'react';
import { useAppSelector } from '@/store';

export function ChatWorkspace({ sidebar, messages, composer, agentPanel }: {
  sidebar: ReactNode;
  messages: ReactNode;
  composer: ReactNode;
  agentPanel: ReactNode;
}) {
  const layout = useAppSelector((state) => state.layout);
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', lg: '250px minmax(420px, 1fr)', xl: `280px minmax(500px, 1fr) ${layout.agent_panel_width}px` }, gap: 1.5, height: { xs: 'auto', lg: 'calc(100vh - 224px)' }, minHeight: 650 }}>
      <Paper component="aside" aria-label="会话列表" variant="outlined" sx={{ overflow: 'hidden', borderRadius: 3, borderColor: '#e1e5ed', boxShadow: '0 8px 24px rgba(31,45,90,.025)' }}>{sidebar}</Paper>
      <Paper component="main" aria-label="消息区" variant="outlined" sx={{ display: 'flex', flexDirection: 'column', overflow: 'hidden', borderRadius: 3, borderColor: '#e1e5ed', boxShadow: '0 8px 24px rgba(31,45,90,.025)', minHeight: 650 }}>
        {messages}{composer}
      </Paper>
      <Paper component="aside" aria-label="Agent 运行面板" variant="outlined" sx={{ display: { xs: 'none', xl: 'block' }, overflow: 'hidden', position: 'relative', borderRadius: 3, borderColor: '#e1e5ed', boxShadow: '0 8px 24px rgba(31,45,90,.025)' }}>
        {agentPanel}
      </Paper>
    </Box>
  );
}
