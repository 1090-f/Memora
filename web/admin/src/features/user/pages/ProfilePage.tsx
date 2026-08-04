import { Paper, Typography } from '@mui/material';
export function ProfilePage() {
  return (
    <Paper variant="outlined" sx={{ p: 3 }}>
      <Typography component="h2" variant="h5" fontWeight={750}>个人资料</Typography>
      <Typography color="text.secondary" mt={1}>用户资料接口已具备，表单将在认证任务中接入。</Typography>
    </Paper>
  );
}
