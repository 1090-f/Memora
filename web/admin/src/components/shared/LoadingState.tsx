import { CircularProgress, Stack, Typography } from '@mui/material';

export function LoadingState({ label = '正在加载' }: { label?: string }) {
  return (
    <Stack alignItems="center" justifyContent="center" spacing={2} py={8} role="status">
      <CircularProgress size={28} />
      <Typography color="text.secondary">{label}</Typography>
    </Stack>
  );
}
