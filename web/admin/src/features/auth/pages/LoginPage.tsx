import { Box, Paper, Stack, Typography } from '@mui/material';

export function LoginPage() {
  return (
    <Box sx={{ minHeight: '100vh', display: 'grid', placeItems: 'center', p: 3 }}>
      <Paper elevation={0} sx={{ width: 'min(440px, 100%)', p: 5, border: '1px solid #e5e6ea' }}>
        <Stack spacing={1}>
          <Typography component="h1" variant="h4" fontWeight={800}>登录 Memora</Typography>
          <Typography color="text.secondary">认证表单将在当前用户接口接入阶段启用。</Typography>
        </Stack>
      </Paper>
    </Box>
  );
}
