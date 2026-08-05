import AccountCircleOutlined from '@mui/icons-material/AccountCircleOutlined';
import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import BuildOutlined from '@mui/icons-material/BuildOutlined';
import LogoutOutlined from '@mui/icons-material/LogoutOutlined';
import MemoryOutlined from '@mui/icons-material/MemoryOutlined';
import MenuBookOutlined from '@mui/icons-material/MenuBookOutlined';
import SettingsOutlined from '@mui/icons-material/SettingsOutlined';
import TimelineOutlined from '@mui/icons-material/TimelineOutlined';
import {
  AppBar,
  Box,
  Button,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Toolbar,
  Typography,
} from '@mui/material';
import type { ReactNode } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { logout } from '@/features/auth/api';
import { useAppDispatch, useAppSelector } from '@/store';
import { clearAuth } from '@/store/authSlice';

const navigation: Array<{ label: string; path: string; icon: ReactNode }> = [
  { label: '知识库', path: '/knowledge-bases', icon: <MenuBookOutlined /> },
  { label: '运行记录', path: '/runs', icon: <TimelineOutlined /> },
  { label: '长期记忆', path: '/memories', icon: <MemoryOutlined /> },
  { label: 'MCP 工具', path: '/mcp', icon: <BuildOutlined /> },
  { label: '模型设置', path: '/settings/models', icon: <AutoAwesomeOutlined /> },
  { label: '个人资料', path: '/settings/profile', icon: <AccountCircleOutlined /> },
];

const pageTitles: Record<string, string> = {
  '/knowledge-bases': '知识库',
  '/runs': 'Agent 运行记录',
  '/memories': '长期记忆',
  '/mcp': 'MCP 工具',
  '/settings/models': '模型设置',
  '/settings/profile': '个人资料',
};

export function AppShell() {
  const location = useLocation();
  const navigate = useNavigate();
  const dispatch = useAppDispatch();
  const user = useAppSelector((state) => state.auth.user);
  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      // Local authentication state is authoritative after the user requests logout.
    } finally {
      dispatch(clearAuth());
      navigate('/login', { replace: true });
    }
  };
  const title =
    pageTitles[location.pathname] ||
    (location.pathname.startsWith('/chat') ? '智能问答' : undefined) ||
    (location.pathname.includes('/docs') ? '文档工作区' : undefined) ||
    (location.pathname.includes('/search-test') ? '检索测试' : undefined) ||
    (location.pathname.includes('/settings') ? '知识库设置' : 'Memora');

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>
      <Box
        component="aside"
        sx={{
          position: 'fixed',
          inset: '0 auto 0 0',
          width: 224,
          bgcolor: '#17181c',
          color: '#fff',
          zIndex: 1201,
        }}
      >
        <Toolbar sx={{ minHeight: '64px !important', px: 2.5 }}>
          <SettingsOutlined sx={{ color: '#8d8df2', mr: 1.25 }} />
          <Typography fontWeight={800} letterSpacing={0.3}>Memora</Typography>
        </Toolbar>
        <List component="nav" aria-label="主导航" sx={{ px: 1.25, pt: 2 }}>
          {navigation.map((item) => (
            <ListItemButton
              key={item.path}
              component={NavLink}
              to={item.path}
              selected={location.pathname === item.path}
              sx={{
                borderRadius: 2,
                color: '#c7c9cf',
                mb: 0.5,
                '&.Mui-selected': { bgcolor: '#31323a', color: '#fff' },
                '&:focus-visible': { outline: '2px solid #a5a5ff', outlineOffset: 2 },
              }}
            >
              <ListItemIcon sx={{ minWidth: 38, color: 'inherit' }}>{item.icon}</ListItemIcon>
              <ListItemText primary={item.label} />
            </ListItemButton>
          ))}
        </List>
      </Box>

      <AppBar
        position="fixed"
        elevation={0}
        color="inherit"
        sx={{ left: 224, width: 'calc(100% - 224px)', borderBottom: '1px solid #e6e7eb' }}
      >
        <Toolbar sx={{ minHeight: '64px !important' }}>
          <Typography component="h1" variant="h6" fontWeight={750}>{title}</Typography>
          <Box sx={{ flexGrow: 1 }} />
          {user && <Typography color="text.secondary" sx={{ mr: 2 }}>{user.nickname}</Typography>}
          <Button
            color="inherit"
            startIcon={<LogoutOutlined />}
            onClick={() => void handleLogout()}
          >
            退出登录
          </Button>
        </Toolbar>
      </AppBar>

      <Box component="main" sx={{ ml: '224px', pt: '64px', minWidth: 800 }}>
        <Box sx={{ display: { xs: 'block', lg: 'none' }, bgcolor: '#fff3cd', px: 3, py: 1 }}>
          建议使用 1280px 或更宽的桌面窗口，以获得完整工作台体验。
        </Box>
        <Box sx={{ p: 3 }}><Outlet /></Box>
      </Box>
    </Box>
  );
}
