import { Box, Typography } from '@mui/material';
import InboxOutlined from '@mui/icons-material/InboxOutlined';
import type { ReactNode } from 'react';

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <Box sx={{ py: 8, textAlign: 'center' }}>
      <InboxOutlined sx={{ color: 'text.disabled', fontSize: 42 }} />
      <Typography variant="h6" mt={2}>{title}</Typography>
      <Typography color="text.secondary" mt={1}>{description}</Typography>
      {action && <Box mt={3}>{action}</Box>}
    </Box>
  );
}
