import { Box } from '@mui/material';
import { Outlet } from 'react-router-dom';

export function AnonymousLayout() {
  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#f2f3f8' }}>
      <Outlet />
    </Box>
  );
}
