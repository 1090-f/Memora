import { createSlice, type PayloadAction } from '@reduxjs/toolkit';

const STORAGE_KEY = 'memora.layout.chat';
export interface LayoutState { agent_panel_width: number; agent_panel_collapsed: boolean }
const clamp = (width: number) => Math.min(480, Math.max(320, width));

export const readChatLayout = (): LayoutState => {
  try {
    const raw = JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}') as Partial<LayoutState>;
    return { agent_panel_width: clamp(raw.agent_panel_width ?? 360), agent_panel_collapsed: raw.agent_panel_collapsed ?? false };
  } catch {
    return { agent_panel_width: 360, agent_panel_collapsed: false };
  }
};

export const persistChatLayout = (state: LayoutState) => localStorage.setItem(STORAGE_KEY, JSON.stringify(state));

const layoutSlice = createSlice({
  name: 'layout',
  initialState: readChatLayout,
  reducers: {
    setAgentPanelWidth(state, action: PayloadAction<number>) { state.agent_panel_width = clamp(action.payload); },
    setAgentPanelCollapsed(state, action: PayloadAction<boolean>) { state.agent_panel_collapsed = action.payload; },
  },
});

export const { setAgentPanelWidth, setAgentPanelCollapsed } = layoutSlice.actions;
export default layoutSlice.reducer;
