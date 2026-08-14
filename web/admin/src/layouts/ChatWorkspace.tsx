import { ChevronLeft, ChevronRight } from '@mui/icons-material';
import { Box, IconButton, Paper, Tooltip } from '@mui/material';
import { useState, type ReactNode } from 'react';
import { useAppSelector } from '@/store';

export function ChatWorkspace({ sidebar, messages, composer, agentPanel }: {
  sidebar: ReactNode;
  messages: ReactNode;
  composer: ReactNode;
  agentPanel: ReactNode;
}) {
  const layout = useAppSelector((state) => state.layout);
  // 折叠状态：true 表示已折叠（只显示窄条 + 展开按钮）
  const [collapsed, setCollapsed] = useState(false);

  // 折叠后的窄栏宽度（仅放展开按钮），展开时使用配置的宽度
  const COLLAPSED_WIDTH = 36;
  const panelWidth = collapsed ? COLLAPSED_WIDTH : layout.agent_panel_width;

  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: `280px minmax(420px, 1fr) ${panelWidth}px`, gap: 1.5, height: 'calc(100vh - 128px)', minHeight: 620, transition: 'grid-template-columns 0.25s ease' }}>
      <Paper component="aside" aria-label="会话列表" variant="outlined" sx={{ overflow: 'hidden' }}>{sidebar}</Paper>
      <Paper component="main" aria-label="消息区" variant="outlined" sx={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {messages}{composer}
      </Paper>
      <Paper component="aside" aria-label="Agent 运行面板" variant="outlined" sx={{ overflow: 'hidden', position: 'relative', display: 'flex', flexDirection: 'column' }}>
        {/* 折叠/展开按钮 — 始终显示在右上角 */}
        <Tooltip title={collapsed ? '展开' : '折叠'} placement="left">
          <IconButton
            size="small"
            onClick={() => setCollapsed((prev) => !prev)}
            sx={{ position: 'absolute', top: 6, right: 4, zIndex: 1, width: 28, height: 28 }}
          >
            {collapsed ? <ChevronLeft sx={{ fontSize: '1.1rem' }} /> : <ChevronRight sx={{ fontSize: '1.1rem' }} />}
          </IconButton>
        </Tooltip>

        {collapsed ? (
          /* 折叠态：仅显示窄条和展开按钮 */
          <Box sx={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center' }}>
            {/* 留白 */}
          </Box>
        ) : (
          /* 展开态：正常显示 Agent 面板内容 */
          agentPanel
        )}
      </Paper>
    </Box>
  );
}
