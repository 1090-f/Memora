import { Box, Paper } from '@mui/material';
import type { ReactNode } from 'react';

export function DocumentWorkspace({ sidebar, children }: { sidebar: ReactNode; children: ReactNode }) {
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: '280px minmax(0, 1fr)', gap: 2 }}>
      <Paper component="aside" variant="outlined" sx={{ minHeight: 560, overflow: 'auto' }}>{sidebar}</Paper>
      <Box>{children}</Box>
    </Box>
  );
}
