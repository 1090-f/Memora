import { Box, Paper } from '@mui/material';
import type { ReactNode } from 'react';
import { useAppDispatch, useAppSelector } from '@/store';
import { persistChatLayout, setAgentPanelWidth } from '@/store/layoutSlice';

export function ChatWorkspace({ sidebar, messages, composer, agentPanel }: {
  sidebar: ReactNode;
  messages: ReactNode;
  composer: ReactNode;
  agentPanel: ReactNode;
}) {
  const dispatch = useAppDispatch();
  const layout = useAppSelector((state) => state.layout);
  const updateWidth = (raw: number) => {
    const agent_panel_width = Math.min(480, Math.max(320, raw));
    const next = { ...layout, agent_panel_width };
    dispatch(setAgentPanelWidth(agent_panel_width));
    persistChatLayout(next);
  };
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: `280px minmax(420px, 1fr) ${layout.agent_panel_width}px`, gap: 1.5, height: 'calc(100vh - 128px)', minHeight: 620 }}>
      <Paper component="aside" aria-label="会话列表" variant="outlined" sx={{ overflow: 'hidden' }}>{sidebar}</Paper>
      <Paper component="main" aria-label="消息区" variant="outlined" sx={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {messages}{composer}
      </Paper>
      <Paper component="aside" aria-label="Agent 运行面板" variant="outlined" sx={{ overflow: 'hidden', position: 'relative' }}>
        <input
          aria-label="Agent 面板宽度"
          type="range"
          min="320"
          max="480"
          value={layout.agent_panel_width}
          onChange={(event) => updateWidth(Number(event.target.value))}
          style={{ width: 'calc(100% - 32px)', margin: '12px 16px 0' }}
        />
        {agentPanel}
      </Paper>
    </Box>
  );
}
